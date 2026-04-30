package mkcert

import (
	"fmt"
	"io"
	"log"
	"os/exec"
)

const (
	MKCERT = "mkcert"
)

type MkCert interface {
	CARoot() (string, error)
	Install() error
	Uninstall() error
	Generate(args ...string) error
}

type mkcert struct {
	bin string
	l   *log.Logger
}

func New(logSink io.Writer) MkCert {
	logger := log.New(logSink, "", log.LstdFlags)
	return &mkcert{bin: MKCERT, l: logger}
}

func (m *mkcert) CARoot() (string, error) {
	out, err := exec.Command(m.bin, "-CAROOT").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to mkcert -CAROOT: %v %v", string(out), err)
	}
	return string(out), nil
}

func (m *mkcert) Install() error {
	out, err := exec.Command(m.bin, "--install").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mkcert -install: %v %v", string(out), err)
	}
	m.l.Print(string(out))
	return nil
}

func (m *mkcert) Uninstall() error {
	out, err := exec.Command(m.bin, "-uninstall").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mkcert -uninstall: %v %v", string(out), err)
	}
	m.l.Print(string(out))
	return nil
}

func (m *mkcert) Generate(args ...string) error {
	out, err := exec.Command(m.bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mkcert -CAROOT: %v %v", string(out), err)
	}
	m.l.Print(string(out))
	return nil
}
