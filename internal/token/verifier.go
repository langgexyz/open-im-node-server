package token

// Verifier wraps VerifyUserToken with a fixed expected signer (node public key).
type Verifier struct {
	expectedSigner string
}

// NewVerifier creates a Verifier that checks tokens against the given node public key address.
func NewVerifier(nodePublicKey string) *Verifier {
	return &Verifier{expectedSigner: nodePublicKey}
}

// Verify validates the user token and returns its payload.
func (v *Verifier) Verify(tokenStr string) (*UserTokenPayload, error) {
	return VerifyUserToken(tokenStr, v.expectedSigner)
}
