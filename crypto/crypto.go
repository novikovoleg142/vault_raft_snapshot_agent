package crypto

import (
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp"
)

type Crypto struct {
	keyPass []byte
}
type KeyWriter io.Writer

type CryptoInterface interface {
	Encrypt(in io.Reader, out io.Writer) error
}

var (
	noSymmetric = fmt.Errorf("Symmetric not set")
	wrongPass   = fmt.Errorf("Wrong password")
)
var _ CryptoInterface = &Crypto{}

func NewCrypto(pwd []byte) (*Crypto, error) {
	return &Crypto{
		keyPass: pwd,
	}, nil
}
func (cr *Crypto) Encrypt(in io.Reader, out io.Writer) error {
	w, err := openpgp.SymmetricallyEncrypt(out, cr.keyPass, nil, nil)
	if err != nil {
		return fmt.Errorf("can't encrypt the message: %w", err)
	}
	if _, err = io.Copy(w, in); err != nil {
		return fmt.Errorf("can't copy encrypted message: %w", err)
	}
	return w.Close()
}
