package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"strings"
	"testing"
)

// Diskteki yerleşimi bayt bayt doğruluyoruz. Bu test formatı sabitler:
// birisi ileride alanların sırasını veya boyutunu değiştirirse burada patlar.
func TestEncodeRecordLayout(t *testing.T) {
	rec, err := encodeRecord(OpSet, "ab", []byte("xyz"))
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}

	wantLen := HeaderSize + 2 + 3
	if len(rec) != wantLen {
		t.Fatalf("kayıt uzunluğu %d, beklenen %d", len(rec), wantLen)
	}

	if got := Op(rec[4]); got != OpSet {
		t.Errorf("op = %d, beklenen %d", got, OpSet)
	}
	if got := binary.LittleEndian.Uint32(rec[5:9]); got != 2 {
		t.Errorf("keyLen = %d, beklenen 2", got)
	}
	if got := binary.LittleEndian.Uint32(rec[9:13]); got != 3 {
		t.Errorf("valLen = %d, beklenen 3", got)
	}
	if got := string(rec[13:15]); got != "ab" {
		t.Errorf("key = %q, beklenen %q", got, "ab")
	}
	if got := string(rec[15:18]); got != "xyz" {
		t.Errorf("value = %q, beklenen %q", got, "xyz")
	}

	// CRC, kendisi hariç kaydın tamamını kapsamalı.
	wantCRC := crc32.ChecksumIEEE(rec[4:])
	if got := binary.LittleEndian.Uint32(rec[0:4]); got != wantCRC {
		t.Errorf("CRC = %d, beklenen %d", got, wantCRC)
	}
}

// Tek bir baytı bozmak CRC'yi mutlaka değiştirmeli — kurtarmanın bozuk kaydı
// yakalayabilmesi tamamen buna bağlı.
func TestCRCDetectsCorruption(t *testing.T) {
	rec, err := encodeRecord(OpSet, "key", []byte("value"))
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	stored := binary.LittleEndian.Uint32(rec[0:4])

	for _, pos := range []int{4, 5, 9, 13, HeaderSize + 3} {
		corrupt := make([]byte, len(rec))
		copy(corrupt, rec)
		corrupt[pos]++ // tek bayt bozuyoruz

		if crc32.ChecksumIEEE(corrupt[4:]) == stored {
			t.Errorf("%d. bayt bozulduğu hâlde CRC değişmedi", pos)
		}
	}
}

// Değerin içinde satır sonu ve sıfır baytı olması hiçbir şeyi bozmamalı:
// uzunluk öneki kullanmamızın bütün sebebi bu.
func TestEncodeRecordBinarySafeValue(t *testing.T) {
	value := []byte("satir1\nsatir2\x00\xffSET sahte anahtar\n")

	rec, err := encodeRecord(OpSet, "k", value)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}

	valLen := binary.LittleEndian.Uint32(rec[9:13])
	if int(valLen) != len(value) {
		t.Fatalf("valLen = %d, beklenen %d", valLen, len(value))
	}
	if got := rec[HeaderSize+1:]; string(got) != string(value) {
		t.Errorf("value bozuldu: %q", got)
	}
}

func TestEncodeRecordDeleteHasNoValue(t *testing.T) {
	rec, err := encodeRecord(OpDelete, "k", nil)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}

	if len(rec) != HeaderSize+1 {
		t.Fatalf("kayıt uzunluğu %d, beklenen %d", len(rec), HeaderSize+1)
	}
	if got := binary.LittleEndian.Uint32(rec[9:13]); got != 0 {
		t.Errorf("valLen = %d, beklenen 0", got)
	}
}

func TestEncodeRecordRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		op    Op
		key   string
		value []byte
		want  error
	}{
		{"bilinmeyen op", Op(99), "k", nil, ErrInvalidOp},
		{"sıfır op", Op(0), "k", nil, ErrInvalidOp},
		{"çok büyük key", OpSet, strings.Repeat("a", MaxKeySize+1), nil, ErrKeyTooLarge},
		{"çok büyük value", OpSet, "k", make([]byte, MaxValueSize+1), ErrValueTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := encodeRecord(tt.op, tt.key, tt.value); !errors.Is(err, tt.want) {
				t.Fatalf("hata = %v, beklenen %v", err, tt.want)
			}
		})
	}
}

// Sınırın tam üstündeki değer reddedilirken tam sınırdaki kabul edilmeli.
func TestEncodeRecordAcceptsExactLimits(t *testing.T) {
	if _, err := encodeRecord(OpSet, strings.Repeat("a", MaxKeySize), nil); err != nil {
		t.Errorf("tam sınırdaki key reddedildi: %v", err)
	}
	if _, err := encodeRecord(OpSet, "k", make([]byte, MaxValueSize)); err != nil {
		t.Errorf("tam sınırdaki value reddedildi: %v", err)
	}
}

// Boş key ve boş value geçerli olmalı: store da bunlara izin veriyor,
// iki katman arasında kural farkı olmasın.
func TestEncodeRecordAllowsEmpty(t *testing.T) {
	rec, err := encodeRecord(OpSet, "", []byte{})
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if len(rec) != HeaderSize {
		t.Fatalf("kayıt uzunluğu %d, beklenen %d", len(rec), HeaderSize)
	}
}
