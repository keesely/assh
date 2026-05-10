package sftp

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const (
	HeaderMagic   = 0x41535348
	HeaderVersion  = 1
	HeaderSize     = 256
	HashTypeSHA256 = 1

	hashTypeSize = 2
	origSizeSize = 8
	sha256Size   = 32
)

var (
	ErrInvalidHeader = errors.New("invalid hash header")
)

type HashHeader struct {
	Version  uint16
	HashType uint16
	OrigSize int64
	SHA256   [32]byte
	Reserved [HeaderSize - 4 - 2 - 2 - 8 - 32]byte
}

func BuildHashHeader(origSize int64, sha256Hash []byte) []byte {
	header := HashHeader{
		Version:  HeaderVersion,
		HashType: HashTypeSHA256,
		OrigSize: origSize,
	}
	copy(header.SHA256[:], sha256Hash[:32])

	buf := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(buf[0:4], HeaderMagic)
	binary.BigEndian.PutUint16(buf[4:6], header.Version)
	binary.BigEndian.PutUint16(buf[6:8], header.HashType)
	binary.BigEndian.PutUint64(buf[8:16], uint64(header.OrigSize))
	copy(buf[16:48], header.SHA256[:])
	return buf
}

func ParseHashHeader(data []byte) (*HashHeader, error) {
	if len(data) < HeaderSize {
		return nil, ErrInvalidHeader
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != HeaderMagic {
		return nil, ErrInvalidHeader
	}

	header := &HashHeader{
		Version:  binary.BigEndian.Uint16(data[4:6]),
		HashType: binary.BigEndian.Uint16(data[6:8]),
		OrigSize: int64(binary.BigEndian.Uint64(data[8:16])),
	}
	copy(header.SHA256[:], data[16:48])

	return header, nil
}

func StripHeader(r io.Reader, w io.Writer, totalSize int64) error {
	if totalSize <= HeaderSize {
		return ErrInvalidHeader
	}

	hashHeader := make([]byte, HeaderSize)
	n, err := r.Read(hashHeader)
	if err != nil && err != io.EOF {
		return err
	}
	if n < HeaderSize {
		return ErrInvalidHeader
	}

	parsed, err := ParseHashHeader(hashHeader)
	if err != nil {
		return err
	}

	actualContentSize := totalSize - HeaderSize
	if parsed.OrigSize != actualContentSize {
		return ErrInvalidHeader
	}

	remaining := totalSize - HeaderSize
	buf := make([]byte, 32*1024)
	for remaining > 0 {
		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}
		n, err := r.Read(buf[:toRead])
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		_, err = w.Write(buf[:n])
		if err != nil {
			return err
		}
		remaining -= int64(n)
	}

	return nil
}

func AddHeader(r io.Reader, w io.Writer, sha256Hash []byte, origSize int64) error {
	header := BuildHashHeader(origSize, sha256Hash)
	_, err := w.Write(header)
	if err != nil {
		return err
	}

	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(w, r, buf)
	return err
}

func VerifyLocalHash(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}