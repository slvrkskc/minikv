package wal

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

// SyncMode, bir yazmanın ne zaman fiziksel diske indiğini belirler.
//
// Anlaşılması gereken üç katman var:
//
//  1. bufio arabelleği — veri hâlâ bizim süreç belleğimizde. Süreç çökerse gider.
//  2. İşletim sistemi sayfa önbelleği (page cache) — write() çağrısı buraya kadar
//     getirir. Süreç çökse bile veri hayatta kalır, ama elektrik kesilirse gider.
//  3. Fiziksel disk — ancak fsync() çağrısı bunu garanti eder.
//
// fsync pahalıdır (SSD'de bile yüz mikrosaniyeler, HDD'de milisaniyeler), bu
// yüzden "her yazmada fsync" ile "hızlı ama riskli" arasında seçim yapmak
// gerekir. Redis'in appendfsync ayarı tam olarak bu seçimi sunar.
type SyncMode int

const (
	// SyncNever: fsync yalnızca Sync() veya Close() çağırınca yapılır.
	// En hızlı, en az dayanıklı (durable).
	SyncNever SyncMode = iota
	// SyncAlways: her kayıttan sonra fsync. En yavaş, en dayanıklı.
	SyncAlways
)

// WAL, append-only bir günlük dosyasına yazar. Eşzamanlı kullanıma güvenlidir.
type WAL struct {
	mu   sync.Mutex
	f    *os.File
	bw   *bufio.Writer
	mode SyncMode
	size int64 // dosyanın mantıksal boyutu; compaction kararı için gerekecek
}

// Open, verilen yoldaki günlüğü yazmak üzere açar. Dosya yoksa oluşturur,
// varsa sonuna ekler (üzerine YAZMAZ — günlüğün tamamı tarihçedir).
func Open(path string, mode SyncMode) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("wal: dosya açılamadı: %w", err)
	}

	// Mevcut boyutu öğreniyoruz, çünkü dosyanın sonuna ekliyoruz.
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("wal: dosya bilgisi okunamadı: %w", err)
	}

	return &WAL{
		f:    f,
		bw:   bufio.NewWriter(f),
		mode: mode,
		size: info.Size(),
	}, nil
}

// AppendSet, bir SET işlemini günlüğe ekler.
func (w *WAL) AppendSet(key string, value []byte) error {
	return w.append(OpSet, key, value)
}

// AppendDelete, bir DEL işlemini günlüğe ekler.
//
// Silmeyi de yazmak zorundayız: aksi halde kurtarma sırasında günlükteki eski
// SET kaydını görüp silinmiş anahtarı geri diriltirdik. Bu kayda literatürde
// "tombstone" (mezar taşı) denir.
func (w *WAL) AppendDelete(key string) error {
	return w.append(OpDelete, key, nil)
}

func (w *WAL) append(op Op, key string, value []byte) error {
	rec, err := encodeRecord(op, key, value)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.bw.Write(rec)
	w.size += int64(n)
	if err != nil {
		return fmt.Errorf("wal: yazma başarısız: %w", err)
	}

	if w.mode == SyncAlways {
		return w.syncLocked()
	}
	return nil
}

// Sync, arabellekteki her şeyi diske indirir ve fsync çağırır.
// Bu çağrı döndükten sonra veri elektrik kesintisine karşı güvendedir.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncLocked()
}

// syncLocked, w.mu kilitliyken çağrılmalıdır.
func (w *WAL) syncLocked() error {
	// Sıra önemli: önce bizim arabelleğimiz işletim sistemine boşalır,
	// sonra işletim sistemi diske yazmaya zorlanır. Ters sırada yapılsa
	// arabellekte bekleyen kayıtlar fsync'e hiç yetişmezdi.
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("wal: arabellek boşaltılamadı: %w", err)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("wal: fsync başarısız: %w", err)
	}
	return nil
}

// Size, günlüğün bayt cinsinden boyutunu döner.
func (w *WAL) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

// Close, bekleyen her şeyi diske indirip dosyayı kapatır.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	syncErr := w.syncLocked()
	closeErr := w.f.Close()

	// Sync hatası daha bilgilendirici, onu önceliyoruz; ama dosyayı her
	// durumda kapatıyoruz ki tanıtıcı (file descriptor) sızmasın.
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return fmt.Errorf("wal: dosya kapatılamadı: %w", closeErr)
	}
	return nil
}
