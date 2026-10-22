package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type parsedRecord struct {
	op    Op
	key   string
	value []byte
}

// parseAll, günlük dosyasının tamamını kayıtlara ayırır ve her birinin CRC'sini
// doğrular. Adım 3'te bunun gerçek sürümünü yazacağız; şimdilik testin
// formatı bağımsızca okuduğunu göstermek için burada duruyor.
func parseAll(t *testing.T, data []byte) []parsedRecord {
	t.Helper()

	var out []parsedRecord
	for off := 0; off < len(data); {
		if len(data)-off < HeaderSize {
			t.Fatalf("offset %d: başlık için yeterli bayt yok", off)
		}
		header := data[off : off+HeaderSize]
		storedCRC := binary.LittleEndian.Uint32(header[0:4])
		op := Op(header[4])
		keyLen := int(binary.LittleEndian.Uint32(header[5:9]))
		valLen := int(binary.LittleEndian.Uint32(header[9:13]))

		end := off + HeaderSize + keyLen + valLen
		if end > len(data) {
			t.Fatalf("offset %d: gövde dosyanın dışına taşıyor", off)
		}
		body := data[off+4 : end]
		if got := crc32.ChecksumIEEE(body); got != storedCRC {
			t.Fatalf("offset %d: CRC uyuşmuyor (%d != %d)", off, got, storedCRC)
		}

		keyStart := off + HeaderSize
		out = append(out, parsedRecord{
			op:    op,
			key:   string(data[keyStart : keyStart+keyLen]),
			value: data[keyStart+keyLen : end],
		})
		off = end
	}
	return out
}

func tempWAL(t *testing.T, mode SyncMode) (*WAL, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.wal")
	w, err := Open(path, mode)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	return w, path
}

func TestOpenCreatesFile(t *testing.T) {
	_, path := tempWAL(t, SyncNever)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("günlük dosyası oluşmadı: %v", err)
	}
}

func TestAppendAndSync(t *testing.T) {
	w, path := tempWAL(t, SyncNever)

	if err := w.AppendSet("isim", []byte("alara")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}
	if err := w.AppendSet("sehir", []byte("İstanbul")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}
	if err := w.AppendDelete("isim"); err != nil {
		t.Fatalf("AppendDelete: %v", err)
	}
	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	recs := parseAll(t, data)

	want := []parsedRecord{
		{OpSet, "isim", []byte("alara")},
		{OpSet, "sehir", []byte("İstanbul")},
		{OpDelete, "isim", []byte{}},
	}
	if len(recs) != len(want) {
		t.Fatalf("%d kayıt okundu, beklenen %d", len(recs), len(want))
	}
	for i, wr := range want {
		if recs[i].op != wr.op || recs[i].key != wr.key || string(recs[i].value) != string(wr.value) {
			t.Errorf("kayıt %d = {%d %q %q}, beklenen {%d %q %q}",
				i, recs[i].op, recs[i].key, recs[i].value, wr.op, wr.key, wr.value)
		}
	}
}

// Yazmanın kendisi dayanıklılık (durability) sağlamaz: Sync çağrılmadan önce
// kayıt hâlâ bizim arabelleğimizde bekliyor. Bu testin amacı o farkı
// görünür kılmak.
func TestSyncNeverKeepsDataBuffered(t *testing.T) {
	w, path := tempWAL(t, SyncNever)

	if err := w.AppendSet("k", []byte("v")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Sync'ten önce dosyada %d bayt var, 0 beklenirdi", len(data))
	}

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if data, _ = os.ReadFile(path); len(data) == 0 {
		t.Error("Sync'ten sonra dosya hâlâ boş")
	}
}

// SyncAlways modunda her kayıt anında diske inmeli, Sync çağırmaya gerek yok.
func TestSyncAlwaysWritesImmediately(t *testing.T) {
	w, path := tempWAL(t, SyncAlways)

	if err := w.AppendSet("k", []byte("v")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if recs := parseAll(t, data); len(recs) != 1 {
		t.Fatalf("%d kayıt bulundu, 1 beklenirdi", len(recs))
	}
}

// Günlük append-only: yeniden açmak eski kayıtları silmemeli.
func TestReopenAppendsInsteadOfTruncating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.wal")

	first, err := Open(path, SyncNever)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := first.AppendSet("a", []byte("1")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := Open(path, SyncNever)
	if err != nil {
		t.Fatalf("yeniden Open: %v", err)
	}
	if err := second.AppendSet("b", []byte("2")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	recs := parseAll(t, data)
	if len(recs) != 2 {
		t.Fatalf("%d kayıt bulundu, 2 beklenirdi (eski kayıt silinmiş olabilir)", len(recs))
	}
	if recs[0].key != "a" || recs[1].key != "b" {
		t.Errorf("kayıt sırası yanlış: %q, %q", recs[0].key, recs[1].key)
	}
}

func TestSizeTracksFileGrowth(t *testing.T) {
	w, path := tempWAL(t, SyncNever)

	if w.Size() != 0 {
		t.Fatalf("yeni günlüğün boyutu %d, 0 beklenirdi", w.Size())
	}
	if err := w.AppendSet("key", []byte("value")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}

	want := int64(HeaderSize + len("key") + len("value"))
	if w.Size() != want {
		t.Errorf("Size() = %d, beklenen %d", w.Size(), want)
	}

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != w.Size() {
		t.Errorf("diskteki boyut %d, Size() %d döndürdü", info.Size(), w.Size())
	}
}

// Reddedilen bir kayıt dosyaya hiç yazılmamalı ve boyutu değiştirmemeli.
func TestRejectedRecordDoesNotTouchLog(t *testing.T) {
	w, _ := tempWAL(t, SyncNever)

	if err := w.AppendSet("k", make([]byte, MaxValueSize+1)); err == nil {
		t.Fatal("çok büyük value kabul edildi")
	}
	if w.Size() != 0 {
		t.Errorf("reddedilen kayıttan sonra boyut %d, 0 beklenirdi", w.Size())
	}
}

// Birden fazla goroutine aynı anda yazarsa kayıtlar birbirine girmemeli.
// Mutex olmasa dosyada yarım kayıtlar oluşur ve CRC kontrolleri patlar.
func TestConcurrentAppends(t *testing.T) {
	w, path := tempWAL(t, SyncNever)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%02d", i)
			if err := w.AppendSet(key, []byte("deger")); err != nil {
				t.Errorf("AppendSet(%s): %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	if err := w.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// parseAll her kaydın CRC'sini doğruluyor; bozulma olsa oraya takılırdı.
	recs := parseAll(t, data)
	if len(recs) != n {
		t.Fatalf("%d kayıt bulundu, %d beklenirdi", len(recs), n)
	}

	seen := make(map[string]bool, n)
	for _, r := range recs {
		if seen[r.key] {
			t.Errorf("%q iki kez yazılmış", r.key)
		}
		seen[r.key] = true
	}
	for i := 0; i < n; i++ {
		if key := fmt.Sprintf("key-%02d", i); !seen[key] {
			t.Errorf("%q günlükte yok", key)
		}
	}
}

func TestCloseFlushesPendingWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close.wal")

	w, err := Open(path, SyncNever)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.AppendSet("k", []byte("v")); err != nil {
		t.Fatalf("AppendSet: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if recs := parseAll(t, data); len(recs) != 1 {
		t.Fatalf("Close sonrası %d kayıt var, 1 beklenirdi", len(recs))
	}
}
