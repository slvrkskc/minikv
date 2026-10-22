// Package wal, minikv'nin write-ahead log'unu (önden yazma günlüğü) yönetir.
//
// Temel fikir: RAM hızlı ama kalıcı değil, disk kalıcı ama yavaş. Ancak diskin
// SONUNA sırayla eklemek (append) çok hızlıdır. Bu yüzden her değişikliği
// günlüğün sonuna ekleriz; program çökerse günlüğü baştan okuyup aynı duruma
// geri döneriz.
//
// Bu dosya kayıtların disk üzerindeki ikili (binary) biçimini tanımlar:
//
//	+---------+--------+----------+----------+---------+-----------+
//	|  CRC32  |   op   |  keyLen  |  valLen  |   key   |   value   |
//	| 4 bayt  | 1 bayt | 4 bayt   | 4 bayt   | keyLen  |  valLen   |
//	+---------+--------+----------+----------+---------+-----------+
//	          |<------------ CRC bu aralığı korur ---------------->|
//
// Uzunlukları başa yazmamızın sebebi: değerin içinde satır sonu, boşluk veya
// sıfır baytı olabilir. Ayırıcı (delimiter) kullansak bunları escape etmek
// zorunda kalırdık; uzunluk önekiyle hiç aramıyoruz, sadece sayıyoruz.
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Op, bir kaydın hangi işlemi anlattığını söyler.
type Op byte

const (
	// OpSet bilinçli olarak 1'den başlıyor: diskten okuduğumuz sıfır baytlarla
	// dolu bir bölge (ör. yarım kalmış yazma) geçerli bir işlem gibi
	// görünmesin. 0 değeri her zaman "bozuk" demek olsun.
	OpSet    Op = 1
	OpDelete Op = 2
)

// HeaderSize, her kaydın başındaki sabit uzunluklu kısmın boyutu:
// crc(4) + op(1) + keyLen(4) + valLen(4).
const HeaderSize = 13

// Kayıt boyutu sınırları. Bunlara iki sebeple ihtiyaç var: anlamsız büyük
// kayıtları baştan reddetmek, ve kurtarma sırasında bozuk bir uzunluk alanı
// yüzünden gigabaytlık bir ayırma (allocation) yapmamak.
const (
	MaxKeySize   = 1 << 16 // 64 KiB
	MaxValueSize = 1 << 26 // 64 MiB
)

var (
	ErrKeyTooLarge   = errors.New("wal: key boyut sınırını aşıyor")
	ErrValueTooLarge = errors.New("wal: value boyut sınırını aşıyor")
	ErrInvalidOp     = errors.New("wal: bilinmeyen işlem kodu")
	// ErrCorrupt, CRC uyuşmadığında döner: kayıt ya yarım yazılmış ya bozulmuş.
	ErrCorrupt = errors.New("wal: kayıt bozuk (CRC uyuşmuyor)")
)

// encodeRecord, tek bir kaydı diske yazılmaya hazır bayt dizisine çevirir.
func encodeRecord(op Op, key string, value []byte) ([]byte, error) {
	if op != OpSet && op != OpDelete {
		return nil, fmt.Errorf("%w: %d", ErrInvalidOp, op)
	}
	if len(key) > MaxKeySize {
		return nil, fmt.Errorf("%w: %d bayt (en fazla %d)", ErrKeyTooLarge, len(key), MaxKeySize)
	}
	if len(value) > MaxValueSize {
		return nil, fmt.Errorf("%w: %d bayt (en fazla %d)", ErrValueTooLarge, len(value), MaxValueSize)
	}

	buf := make([]byte, HeaderSize+len(key)+len(value))

	// [0:4] CRC'yi en sonda, gövde hazır olunca hesaplayacağız.
	buf[4] = byte(op)
	binary.LittleEndian.PutUint32(buf[5:9], uint32(len(key)))
	binary.LittleEndian.PutUint32(buf[9:13], uint32(len(value)))
	copy(buf[HeaderSize:], key)
	copy(buf[HeaderSize+len(key):], value)

	// CRC, kendisi hariç kaydın tamamını korur.
	binary.LittleEndian.PutUint32(buf[0:4], crc32.ChecksumIEEE(buf[4:]))
	return buf, nil
}
