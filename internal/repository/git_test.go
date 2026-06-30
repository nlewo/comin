package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
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
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for SSH signed commit tests")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen is required for SSH signed commit tests")
	}

	dir = t.TempDir()
	runTestCommand(t, dir, "git", "init", "-q")
	runTestCommand(t, dir, "git", "checkout", "-q", "-b", "main")
	runTestCommand(t, dir, "git", "config", "user.name", "SSH Test")
	runTestCommand(t, dir, "git", "config", "user.email", "ssh@example.com")
	runTestCommand(t, dir, "git", "config", "gpg.format", "ssh")

	signingKeyPath := filepath.Join(dir, "signing_key")
	runTestCommand(t, dir, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "ssh@example.com", "-f", signingKeyPath)
	runTestCommand(t, dir, "git", "config", "user.signingkey", signingKeyPath+".pub")

	filename := filepath.Join(dir, "file-1")
	assert.Nil(t, os.WriteFile(filename, []byte("file-1"), 0644))
	runTestCommand(t, dir, "git", "add", "file-1")
	runTestCommand(t, dir, "git", "commit", "-q", "-S", "-m", "file-1")
	commitId = runTestCommand(t, dir, "git", "rev-parse", "HEAD")

	publicKey, err := os.ReadFile(signingKeyPath + ".pub")
	assert.Nil(t, err)
	allowedSignersPath = filepath.Join(dir, "allowed_signers")
	assert.Nil(t, os.WriteFile(allowedSignersPath, []byte("ssh@example.com "+string(publicKey)), 0644))

	return dir, allowedSignersPath, commitId
}

func runTestCommand(t *testing.T, dir, command string, args ...string) string {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %s\n%s", command, strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output))
}

func testSSHCertificatePublicKey(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen is required for SSH certificate tests")
	}

	dir := t.TempDir()
	caKeyPath := filepath.Join(dir, "ca_key")
	userKeyPath := filepath.Join(dir, "user_key")
	runTestCommand(t, dir, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "ca@example.com", "-f", caKeyPath)
	runTestCommand(t, dir, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "ssh@example.com", "-f", userKeyPath)
	runTestCommand(t, dir, "ssh-keygen", "-q", "-s", caKeyPath, "-I", "test-cert", "-n", "ssh@example.com", userKeyPath+".pub")

	cert, err := os.ReadFile(userKeyPath + "-cert.pub")
	assert.Nil(t, err)
	return strings.TrimSpace(string(cert))
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
	signedBy, err := commitSignedBy(remoteRepository, commitId, []string{string(failPublic), string(testPublic)})
	assert.Nil(t, err)
	assert.Equal(t, "test <test@comin.space>", signedBy.PrimaryIdentity().Name)

	signedBy, err = commitSignedBy(remoteRepository, commitId, []string{string(failPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

	commitId, _ = commitFileAndSign(remoteRepository, dir, "main", "file-2", nil)
	signedBy, err = commitSignedBy(remoteRepository, commitId, []string{string(failPublic), string(testPublic)})
	assert.ErrorContains(t, err, "is not signed")
	assert.Nil(t, signedBy)

}

func TestHeadSignedBySSH(t *testing.T) {
	dir, allowedSignersPath, commitId := initSSHSignedRemoteRepository(t)
	remoteRepository, err := git.PlainOpen(dir)
	assert.Nil(t, err)

	allowedSigners, err := os.ReadFile(allowedSignersPath)
	assert.Nil(t, err)

	signedBy, err := commitSignedBySSH(remoteRepository, commitId, string(allowedSigners))
	assert.Nil(t, err)
	assert.Equal(t, "ssh@example.com", signedBy)

	signedBy, err = commitSignedBySSH(remoteRepository, commitId, "")
	assert.ErrorContains(t, err, "is not signed")
	assert.Equal(t, "", signedBy)
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
