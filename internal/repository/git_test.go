package repository

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

const testSSHPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDL18Zw2FkReAMqtgjNAvTr0il/FmljJnOtEGApGqZcp ssh@example.com"

func commitFile(remoteRepository *git.Repository, dir, branch, content string) (commitId string, err error) {
	return commitFileAndSign(remoteRepository, dir, branch, content, nil)
}

func commitFileAndSign(remoteRepository *git.Repository, dir, branch, content string, signKey *openpgp.Entity) (commitId string, err error) {
	w, err := remoteRepository.Worktree()
	if err != nil {
		return
	}
	_ = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force:  true,
	})

	filename := filepath.Join(dir, content)
	err = os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		return
	}
	_, err = w.Add(content)
	if err != nil {
		return
	}
	hash, err := w.Commit(content, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Unix(0, 0),
		},
		SignKey: signKey,
	})
	if err != nil {
		return
	}
	return hash.String(), nil
}

func initRemoteRepostiory(dir string, initTesting bool) (remoteRepository *git.Repository, err error) {
	remoteRepository, err = git.PlainInit(dir, false)
	if err != nil {
		return
	}

	_, err = commitFile(remoteRepository, dir, "main", "file-1")
	if err != nil {
		return
	}
	_, err = commitFile(remoteRepository, dir, "main", "file-2")
	if err != nil {
		return
	}
	_, err = commitFile(remoteRepository, dir, "main", "file-3")
	if err != nil {
		return
	}

	headRef, err := remoteRepository.Head()
	if err != nil {
		return
	}
	ref := plumbing.NewHashReference("refs/heads/main", headRef.Hash())
	err = remoteRepository.Storer.SetReference(ref)
	if err != nil {
		return
	}
	if initTesting {
		ref = plumbing.NewHashReference("refs/heads/testing", headRef.Hash())
		err = remoteRepository.Storer.SetReference(ref)
		if err != nil {
			return
		}
	}
	return
}

func HeadCommitId(r *git.Repository) string {
	ref, err := r.Head()
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

func initSSHSignedRemoteRepository(t *testing.T) (dir, allowedSignersPath, commitId string) {
	t.Helper()

	dir = t.TempDir()
	remoteRepository, err := git.PlainInit(dir, false)
	assert.Nil(t, err)
	worktree, err := remoteRepository.Worktree()
	assert.Nil(t, err)

	filename := filepath.Join(dir, "file-1")
	assert.Nil(t, os.WriteFile(filename, []byte("file-1"), 0644))
	_, err = worktree.Add("file-1")
	assert.Nil(t, err)
	hash, err := worktree.Commit("file-1", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "SSH Test",
			Email: "ssh@example.com",
			When:  time.Unix(0, 0),
		},
	})
	assert.Nil(t, err)

	commit, err := remoteRepository.CommitObject(hash)
	assert.Nil(t, err)
	signer := testSSHSigner(t)
	signedHash := signCommitWithSSH(t, remoteRepository, commit, signer)
	assert.Nil(t, remoteRepository.Storer.SetReference(
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), signedHash),
	))
	assert.Nil(t, remoteRepository.Storer.SetReference(
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main")),
	))
	commitId = signedHash.String()

	allowedSignersPath = filepath.Join(dir, "allowed_signers")
	assert.Nil(t, os.WriteFile(allowedSignersPath, []byte("ssh@example.com "+string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), 0644))

	return dir, allowedSignersPath, commitId
}

func testSSHSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	assert.Nil(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	assert.Nil(t, err)
	return signer
}

func signCommitWithSSH(t *testing.T, repository *git.Repository, commit *object.Commit, signer ssh.Signer) plumbing.Hash {
	t.Helper()
	encoded := &plumbing.MemoryObject{}
	assert.Nil(t, commit.EncodeWithoutSignature(encoded))
	reader, err := encoded.Reader()
	assert.Nil(t, err)
	defer reader.Close() // nolint:errcheck
	payload, err := io.ReadAll(reader)
	assert.Nil(t, err)

	signedData, err := sshSignedData("git", "", "sha512", payload)
	assert.Nil(t, err)
	signature, err := signer.Sign(rand.Reader, signedData)
	assert.Nil(t, err)
	commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{
		Type: "SSH SIGNATURE",
		Bytes: append([]byte("SSHSIG"), ssh.Marshal(sshSignatureWire{
			Version:       1,
			PublicKey:     signer.PublicKey().Marshal(),
			Namespace:     "git",
			Reserved:      "",
			HashAlgorithm: "sha512",
			Signature:     ssh.Marshal(signature),
		})...),
	}))

	obj := repository.Storer.NewEncodedObject()
	assert.Nil(t, commit.Encode(obj))
	hash, err := repository.Storer.SetEncodedObject(obj)
	assert.Nil(t, err)
	return hash
}

func testSSHCertificatePublicKey(t *testing.T) string {
	t.Helper()
	caSigner := testSSHSigner(t)
	cert := &ssh.Certificate{
		Key:             testSSHSigner(t).PublicKey(),
		Serial:          1,
		CertType:        ssh.UserCert,
		KeyId:           "test-cert",
		ValidPrincipals: []string{"ssh@example.com"},
		ValidBefore:     ssh.CertTimeInfinity,
	}
	assert.Nil(t, cert.SignCert(rand.Reader, caSigner))
	return string(ssh.MarshalAuthorizedKey(cert))
}

func TestIsAncestor(t *testing.T) {
	remoteRepositoryDir := t.TempDir()
	repository, err := initRemoteRepostiory(remoteRepositoryDir, true)
	assert.Nil(t, err)

	iter, err := repository.Log(&git.LogOptions{})
	assert.Nil(t, err)

	commits := make([]object.Commit, 3)
	idx := 0
	_ = iter.ForEach(func(commit *object.Commit) error {
		commits[idx] = *commit
		idx += 1
		return nil
	})

	ret, _ := isAncestor(repository, commits[1].Hash, commits[0].Hash)
	assert.True(t, ret)

	ret, _ = isAncestor(repository, commits[0].Hash, commits[1].Hash)
	assert.False(t, ret)

	ret, _ = isAncestor(repository, commits[0].Hash, commits[0].Hash)
	assert.False(t, ret)

	ret, _ = isAncestor(repository, commits[2].Hash, commits[0].Hash)
	assert.True(t, ret)

	//time.Sleep(100*time.Second)
}

func TestHeadSignedBy(t *testing.T) {
	dir := t.TempDir()
	remoteRepository, _ := git.PlainInit(dir, false)

	r, _ := os.Open("./test.private")
	entityList, _ := openpgp.ReadArmoredKeyRing(r)
	commitId, _ := commitFileAndSign(remoteRepository, dir, "main", "file-1", entityList[0])

	failPublic, _ := os.ReadFile("./fail.public")
	testPublic, _ := os.ReadFile("./test.public")
	signedBy, err := commitSignedByGPG(remoteRepository, commitId, []string{string(failPublic), string(testPublic)})
	assert.Nil(t, err)
	assert.Equal(t, "test <test@comin.space>", signedBy.PrimaryIdentity().Name)

	signedBy, err = commitSignedByGPG(remoteRepository, commitId, []string{string(failPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

	commitId, _ = commitFileAndSign(remoteRepository, dir, "main", "file-2", nil)
	signedBy, err = commitSignedByGPG(remoteRepository, commitId, []string{string(failPublic), string(testPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

}

func TestHeadSignedBySSH(t *testing.T) {
	dir, allowedSignersPath, commitId := initSSHSignedRemoteRepository(t)
	remoteRepository, err := git.PlainOpen(dir)
	assert.Nil(t, err)

	allowedSigners, err := os.ReadFile(allowedSignersPath)
	assert.Nil(t, err)

	signers, err := parseSSHAllowedSigners(string(allowedSigners))
	assert.Nil(t, err)

	signedBy, err := commitSignedBySSH(remoteRepository, commitId, signers)
	assert.Nil(t, err)
	assert.Equal(t, "ssh@example.com", signedBy)

	_, err = parseSSHAllowedSigners("")
	assert.ErrorContains(t, err, "no SSH allowed signers found")
}

func TestSSHSignatureRejectsSHA1Algorithms(t *testing.T) {
	publicKey := testSSHSigner(t).PublicKey()
	for _, format := range []string{"ssh-rsa", "ssh-dss"} {
		signature := pem.EncodeToMemory(&pem.Block{
			Type: "SSH SIGNATURE",
			Bytes: append([]byte("SSHSIG"), ssh.Marshal(sshSignatureWire{
				Version:       1,
				PublicKey:     publicKey.Marshal(),
				Namespace:     "git",
				HashAlgorithm: "sha512",
				Signature:     ssh.Marshal(ssh.Signature{Format: format, Blob: []byte("signature")}),
			})...),
		})
		_, err := parseSSHSignature(string(signature))
		assert.ErrorContains(t, err, "unsupported SSH signature algorithm")
	}
}

func TestSSHAllowedSignersRejectUnsupportedOptions(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{
			name: "unsupported option before namespace",
			line: `ssh@example.com valid-before="20240101",namespaces="git" ` + testSSHPublicKey,
		},
		{
			name: "unsupported option after namespace",
			line: `ssh@example.com namespaces="git",valid-before="20240101" ` + testSSHPublicKey,
		},
		{
			name: "empty option after namespace",
			line: `ssh@example.com namespaces="git", ` + testSSHPublicKey,
		},
		{
			name: "unsupported option in key input",
			line: `ssh@example.com namespaces="git" valid-before="20240101" ` + testSSHPublicKey,
		},
		{
			name: "cert authority option in key input",
			line: `ssh@example.com namespaces="git" cert-authority ` + testSSHPublicKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSSHAllowedSigners(tt.line)
			assert.ErrorContains(t, err, "unsupported SSH allowed signer option")
		})
	}

	_, err := parseSSHAllowedSigners(`ssh@example.com namespaces="git" ` + testSSHPublicKey)
	assert.Nil(t, err)
}

func TestSSHAllowedSignersRejectCertificates(t *testing.T) {
	publicCert := testSSHCertificatePublicKey(t)

	_, err := parseSSHAllowedSigners(`ssh@example.com ` + publicCert)
	assert.ErrorContains(t, err, "SSH certificates are not supported")
}

func TestSSHAllowedSignersRejectUnsupportedPrincipals(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{
			name: "negated principal",
			line: `!ssh@example.com ` + testSSHPublicKey,
		},
		{
			name: "partial wildcard principal",
			line: `*@example.com ` + testSSHPublicKey,
		},
		{
			name: "question wildcard principal",
			line: `ssh?example.com ` + testSSHPublicKey,
		},
		{
			name: "empty principal",
			line: `ssh@example.com, ` + testSSHPublicKey,
		},
		{
			name: "quoted empty principal",
			line: `"" ` + testSSHPublicKey,
		},
		{
			name: "malformed quoted principal",
			line: `""ssh@example.com"" ` + testSSHPublicKey,
		},
		{
			name: "whitespace padded quoted principal",
			line: `" ssh@example.com " ` + testSSHPublicKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSSHAllowedSigners(tt.line)
			assert.ErrorContains(t, err, "unsupported SSH principal pattern")
		})
	}

	_, err := parseSSHAllowedSigners(`* ` + testSSHPublicKey)
	assert.Nil(t, err)
}

func TestSSHAllowedSignersNamespaces(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		allowed   bool
		wantError string
	}{
		{
			name:    "exact git namespace",
			line:    `ssh@example.com namespaces="git" ` + testSSHPublicKey,
			allowed: true,
		},
		{
			name:    "non git namespace",
			line:    `ssh@example.com namespaces="deploy" ` + testSSHPublicKey,
			allowed: false,
		},
		{
			name:    "wildcard namespace",
			line:    `ssh@example.com namespaces="*" ` + testSSHPublicKey,
			allowed: true,
		},
		{
			name:      "unquoted namespace",
			line:      `ssh@example.com namespaces=git ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "unquoted wildcard namespace",
			line:      `ssh@example.com namespaces=* ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "negated git after wildcard",
			line:      `ssh@example.com namespaces="*,!git" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "negated git after exact namespace",
			line:      `ssh@example.com namespaces="git,!git" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "partial wildcard",
			line:      `ssh@example.com namespaces="gi*" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "question wildcard",
			line:      `ssh@example.com namespaces="g?t" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "leading namespace whitespace",
			line:      `ssh@example.com namespaces=" git" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "malformed quoted namespace",
			line:      `ssh@example.com namespaces=""git"" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "duplicate namespace clauses",
			line:      `ssh@example.com namespaces="deploy",namespaces="git" ` + testSSHPublicKey,
			wantError: "duplicate SSH namespaces option",
		},
		{
			name:      "escaped wildcard namespace",
			line:      `ssh@example.com namespaces="\x2a" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
		{
			name:      "escaped git namespace",
			line:      `ssh@example.com namespaces="\x67it" ` + testSSHPublicKey,
			wantError: "unsupported SSH namespace pattern",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signers, err := parseSSHAllowedSigners(tt.line)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.allowed, signers[0].allowsNamespace("git"))
		})
	}
}
