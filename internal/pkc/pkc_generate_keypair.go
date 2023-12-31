package pkc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

// GenerateKeyPair generates a key pair to be used by ssh passphrase-less auth
func GenerateKeyPair() (privateKey, publicKey []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	pub := priv.Public()
	privateKey = pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		},
	)

	sshPublicKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	sshPublicKeyBytes := sshPublicKey.Marshal()
	publicBase64Key := make([]byte, base64.StdEncoding.EncodedLen(len(sshPublicKeyBytes)))
	base64.StdEncoding.Encode(publicBase64Key, sshPublicKeyBytes)
	publicKey = publicBase64Key
	return
}
