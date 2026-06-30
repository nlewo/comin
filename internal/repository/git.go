package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func getRemoteCommitHash(r repository, remote, branch string) *plumbing.Hash {
	remoteBranch := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	remoteHeadRef, err := r.Repository.Reference(
		plumbing.ReferenceName(remoteBranch),
		true)
	if err != nil {
		return nil
	}
	if remoteHeadRef == nil {
		return nil
	}
	commitId := remoteHeadRef.Hash()
	return &commitId
}

func hasNotBeenHardReset(r repository, branchName string, currentMainHash *plumbing.Hash, remoteMainHead *plumbing.Hash) error {
	if currentMainHash != nil && remoteMainHead != nil && *currentMainHash != *remoteMainHead {
		var ok bool
		ok, err := isAncestor(r.Repository, *currentMainHash, *remoteMainHead)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("this branch has been hard reset: its head '%s' is not on top of '%s'",
				remoteMainHead.String(), currentMainHash.String())
		}
	}
	return nil
}

func getHeadFromRemoteAndBranch(r repository, remoteName, branchName, currentMainCommitId string) (newHead plumbing.Hash, msg string, err error) {
	var currentMainHash *plumbing.Hash
	head := getRemoteCommitHash(r, remoteName, branchName)
	if head == nil {
		return newHead, "", fmt.Errorf("the branch '%s/%s' doesn't exist", remoteName, branchName)
	}
	if currentMainCommitId != "" {
		c := plumbing.NewHash(currentMainCommitId)
		currentMainHash = &c
	}

	if err = hasNotBeenHardReset(r, branchName, currentMainHash, head); err != nil {
		return
	}

	commitObject, err := r.Repository.CommitObject(*head)
	if err != nil {
		return
	}

	return *head, commitObject.Message, nil
}

// fetch fetches the config.Remote
func fetch(r repository, remote types.Remote) (err error) {
	logrus.Debugf("Fetching remote '%s'", remote.Name)
	fetchOptions := git.FetchOptions{
		RemoteName: remote.Name,
	}
	// TODO: support several authentication methods
	if remote.Auth.AccessToken != "" {
		fetchOptions.Auth = &http.BasicAuth{
			Username: remote.Auth.Username,
			Password: remote.Auth.AccessToken,
		}
	}

	// TODO: we should get a parent context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remote.Timeout)*time.Second)
	defer cancel()
	// TODO: we should only fetch tracked branches
	err = r.Repository.FetchContext(ctx, &fetchOptions)
	if err == nil {
		logrus.Infof("New commits have been fetched from '%s'", remote.URL)
		return nil
	} else if err != git.NoErrAlreadyUpToDate {
		logrus.Errorf("Pull from remote '%s' failed: %s", remote.Name, err)
		return fmt.Errorf("'git fetch %s' fails: '%s'", remote.Name, err)
	} else {
		logrus.Debugf("No new commits have been fetched from the remote '%s'", remote.Name)
		return nil
	}
}

// isAncestor returns true when the commitId is an ancestor of the branch branchName
func isAncestor(r *git.Repository, base, top plumbing.Hash) (found bool, err error) {
	iter, err := r.Log(&git.LogOptions{From: top})
	if err != nil {
		return false, fmt.Errorf("git log %s fails: '%s'", top, err)
	}

	// To skip the first commit
	isFirst := true
	_ = iter.ForEach(func(commit *object.Commit) error {
		if !isFirst && commit.Hash == base {
			found = true
			// This error is ignored and used to terminate early the loop :/
			return fmt.Errorf("base commit is ancestor of top commit")
		}
		isFirst = false
		return nil
	})
	return
}

func repositoryOpen(config types.GitConfig) (r *git.Repository, err error) {
	// TODO: this block could removed on release v0.16.0
	// This has been introduced to handle the non bare repository, created before v0.13.0.
	// This is required because go-git has some weird behaviors when cloning...
	r, err = git.PlainOpen(config.Path)
	if err == nil {
		cfg, err := r.Config()
		if err == nil {
			if !cfg.Core.IsBare {
				logrus.Infof("git: the repository %s is deleting because it is not a bare repository", config.Path)
				err := os.RemoveAll(config.Path)
				if err != nil {
					logrus.Errorf("git: the repository %s failed to be deleted: ", err)
				}
			}
		}
	}

	r, err = git.PlainInit(config.Path, true)
	if err != nil {
		r, err = git.PlainOpen(config.Path)
		if err != nil {
			return
		}
		logrus.Debugf("The local Git repository located at '%s' has been opened", config.Path)
	} else {
		logrus.Infof("The local Git repository located at '%s' has been initialized", config.Path)
	}
	return
}

func manageRemotes(r *git.Repository, remotes []types.Remote) error {
	for _, remote := range remotes {
		if err := manageRemote(r, remote); err != nil {
			return err
		}
	}
	return nil
}

func manageRemote(r *git.Repository, remote types.Remote) error {
	gitRemote, err := r.Remote(remote.Name)
	if err == git.ErrRemoteNotFound {
		logrus.Infof("Adding remote '%s' with url '%s'", remote.Name, remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: remote.Name,
			URLs: []string{remote.URL},
		})
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}

	remoteConfig := gitRemote.Config()
	if remoteConfig.URLs[0] != remote.URL {
		if err := r.DeleteRemote(remote.Name); err != nil {
			return err
		}
		logrus.Infof("Updating remote %s (%s)", remote.Name, remote.URL)
		_, err = r.CreateRemote(&gitConfig.RemoteConfig{
			Name: remote.Name,
			URLs: []string{remote.URL},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func commitSignedBy(r *git.Repository, commitId string, publicKeys []string) (signedBy *openpgp.Entity, err error) {
	commit, err := r.CommitObject(plumbing.NewHash(commitId))
	if err != nil {
		return nil, err
	}
	for _, k := range publicKeys {
		entity, err := commit.Verify(k)
		if err == nil {
			logrus.Debugf("Commit %s signed by %s", commitId, entity.PrimaryIdentity().Name)
			return entity, nil
		}
	}
	return nil, fmt.Errorf("commit %s is not signed", commitId)
}

func commitSignedByTrustedKey(r *git.Repository, commitId string, gpgPublicKeys []string, sshAllowedSigners string) (string, error) {
	var verifyErr error
	if len(gpgPublicKeys) > 0 {
		entity, err := commitSignedBy(r, commitId, gpgPublicKeys)
		if err == nil {
			return entity.PrimaryIdentity().Name, nil
		}
		verifyErr = err
	}
	if sshAllowedSigners != "" {
		signedBy, err := commitSignedBySSH(r, commitId, sshAllowedSigners)
		if err == nil {
			return signedBy, nil
		}
		verifyErr = err
	}
	if verifyErr != nil {
		return "", verifyErr
	}
	return "", fmt.Errorf("commit %s is not signed", commitId)
}

type sshAllowedSigner struct {
	principal  string
	key        ssh.PublicKey
	namespaces []string
}

func commitSignedBySSH(r *git.Repository, commitId string, allowedSigners string) (string, error) {
	commit, err := r.CommitObject(plumbing.NewHash(commitId))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(allowedSigners) == "" {
		return "", fmt.Errorf("commit %s is not signed", commitId)
	}
	if !strings.HasPrefix(strings.TrimSpace(commit.PGPSignature), "-----BEGIN SSH SIGNATURE-----") {
		return "", fmt.Errorf("commit %s is not signed", commitId)
	}

	signature, err := parseSSHSignature(commit.PGPSignature)
	if err != nil {
		return "", fmt.Errorf("commit %s has an invalid SSH signature: %w", commitId, err)
	}
	if signature.namespace != "git" {
		return "", fmt.Errorf("commit %s has SSH signature namespace %q instead of git", commitId, signature.namespace)
	}

	encoded := &plumbing.MemoryObject{}
	if err := commit.EncodeWithoutSignature(encoded); err != nil {
		return "", err
	}
	reader, err := encoded.Reader()
	if err != nil {
		return "", err
	}
	defer reader.Close() // nolint:errcheck
	payload, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	signedData, err := sshSignedData(signature.namespace, signature.reserved, signature.hashAlgorithm, payload)
	if err != nil {
		return "", err
	}

	signers, err := parseSSHAllowedSigners(allowedSigners)
	if err != nil {
		return "", err
	}
	for _, signer := range signers {
		if !signer.allowsNamespace(signature.namespace) {
			continue
		}
		if !bytes.Equal(signer.key.Marshal(), signature.publicKey) {
			continue
		}
		if err := signer.key.Verify(signedData, signature.signature); err == nil {
			logrus.Debugf("Commit %s signed by %s", commitId, signer.principal)
			return signer.principal, nil
		}
	}
	return "", fmt.Errorf("commit %s is not signed", commitId)
}

func parseSSHAllowedSigners(allowedSigners string) ([]sshAllowedSigner, error) {
	signers := []sshAllowedSigner{}
	for lineNumber, line := range strings.Split(allowedSigners, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		principalField, rest, ok := splitFirstField(line)
		if !ok {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d", lineNumber+1)
		}
		principal, err := sshPrincipalFromPatternList(principalField)
		if err != nil {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: %w", lineNumber+1, err)
		}
		options, keyInput, err := splitSSHAllowedSignersRest(rest)
		if err != nil {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: %w", lineNumber+1, err)
		}
		namespaces, err := sshNamespacesFromOptions(options)
		if err != nil {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: %w", lineNumber+1, err)
		}
		key, _, keyOptions, keyRest, err := ssh.ParseAuthorizedKey([]byte(keyInput))
		if err != nil {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: %w", lineNumber+1, err)
		}
		if len(keyOptions) > 0 {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: unsupported SSH allowed signer option %q", lineNumber+1, keyOptions[0])
		}
		if strings.TrimSpace(string(keyRest)) != "" {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: unsupported trailing data", lineNumber+1)
		}
		if _, ok := key.(*ssh.Certificate); ok {
			return nil, fmt.Errorf("failed to read the SSH allowed signer on line %d: SSH certificates are not supported", lineNumber+1)
		}
		signers = append(signers, sshAllowedSigner{
			principal:  principal,
			key:        key,
			namespaces: namespaces,
		})
	}
	if len(signers) == 0 {
		return nil, fmt.Errorf("no SSH allowed signers found")
	}
	return signers, nil
}

func splitFirstField(line string) (string, string, bool) {
	i := strings.IndexFunc(line, unicode.IsSpace)
	if i < 0 {
		return "", "", false
	}
	return line[:i], strings.TrimLeftFunc(line[i:], unicode.IsSpace), true
}

func splitSSHAllowedSignersRest(rest string) (string, string, error) {
	first, remaining, err := splitSSHToken(rest)
	if err != nil {
		return "", "", err
	}
	if first == "" {
		return "", "", fmt.Errorf("missing SSH public key")
	}
	if isSSHPublicKeyType(first) {
		return "", rest, nil
	}
	if remaining == "" {
		return "", "", fmt.Errorf("missing SSH public key")
	}
	return first, remaining, nil
}

func splitSSHToken(s string) (string, string, error) {
	inQuote := false
	for i, r := range s {
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if unicode.IsSpace(r) && !inQuote {
			return s[:i], strings.TrimLeftFunc(s[i:], unicode.IsSpace), nil
		}
	}
	if inQuote {
		return "", "", fmt.Errorf("unterminated quote")
	}
	return s, "", nil
}

func isSSHPublicKeyType(token string) bool {
	return strings.HasPrefix(token, "ssh-") ||
		strings.HasPrefix(token, "ecdsa-") ||
		strings.HasPrefix(token, "sk-")
}

func sshPrincipalFromPatternList(patternList string) (string, error) {
	principals := strings.Split(patternList, ",")
	for _, principal := range principals {
		if !isSupportedSSHPattern(principal) {
			return "", fmt.Errorf("unsupported SSH principal pattern %q", principal)
		}
	}
	return principals[0], nil
}

func sshNamespacesFromOptions(options string) ([]string, error) {
	var namespaces []string
	if options == "" {
		return nil, nil
	}
	seenNamespaces := false
	optionTokens, err := splitSSHOptionTokens(options)
	if err != nil {
		return nil, err
	}
	for _, option := range optionTokens {
		if strings.HasPrefix(option, "namespaces=") {
			if seenNamespaces {
				return nil, fmt.Errorf("duplicate SSH namespaces option")
			}
			seenNamespaces = true
			value, err := parseSSHOptionValue(strings.TrimPrefix(option, "namespaces="))
			if err != nil {
				return nil, err
			}
			namespaces = []string{}
			for _, namespace := range strings.Split(value, ",") {
				if !isSupportedSSHPattern(namespace) {
					return nil, fmt.Errorf("unsupported SSH namespace pattern %q", namespace)
				}
				namespaces = append(namespaces, namespace)
			}
			continue
		}
		return nil, fmt.Errorf("unsupported SSH allowed signer option %q", option)
	}
	return namespaces, nil
}

func splitSSHOptionTokens(options string) ([]string, error) {
	tokens := []string{}
	start := 0
	inQuote := false
	for i, r := range options {
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			token := options[start:i]
			if token == "" {
				return nil, fmt.Errorf("unsupported SSH allowed signer option %q", token)
			}
			tokens = append(tokens, token)
			start = i + 1
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote")
	}
	token := options[start:]
	if token == "" {
		return nil, fmt.Errorf("unsupported SSH allowed signer option %q", token)
	}
	return append(tokens, token), nil
}

func parseSSHOptionValue(value string) (string, error) {
	if !strings.HasPrefix(value, "\"") || !strings.HasSuffix(value, "\"") || strings.Contains(value, "\\") {
		return "", fmt.Errorf("unsupported SSH namespace pattern %q", value)
	}
	return strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\""), nil
}

func isSupportedSSHPattern(pattern string) bool {
	return pattern != "" && pattern == strings.TrimSpace(pattern) && (pattern == "*" || !strings.ContainsAny(pattern, "*?!\"'"))
}

func (s sshAllowedSigner) allowsNamespace(namespace string) bool {
	if len(s.namespaces) == 0 {
		return true
	}
	for _, allowed := range s.namespaces {
		if allowed == namespace || allowed == "*" {
			return true
		}
	}
	return false
}

type sshCommitSignature struct {
	publicKey     []byte
	namespace     string
	reserved      string
	hashAlgorithm string
	signature     *ssh.Signature
}

func parseSSHSignature(signature string) (*sshCommitSignature, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(signature)))
	if block == nil || block.Type != "SSH SIGNATURE" {
		return nil, fmt.Errorf("not an SSH signature")
	}
	if !bytes.HasPrefix(block.Bytes, []byte("SSHSIG")) {
		return nil, fmt.Errorf("missing SSH signature magic")
	}

	var wire struct {
		Version       uint32
		PublicKey     []byte
		Namespace     string
		Reserved      string
		HashAlgorithm string
		Signature     []byte
	}
	if err := ssh.Unmarshal(block.Bytes[len("SSHSIG"):], &wire); err != nil {
		return nil, err
	}
	if wire.Version != 1 {
		return nil, fmt.Errorf("unsupported SSH signature version %d", wire.Version)
	}
	if _, err := ssh.ParsePublicKey(wire.PublicKey); err != nil {
		return nil, err
	}

	sshSignature := &ssh.Signature{}
	if err := ssh.Unmarshal(wire.Signature, sshSignature); err != nil {
		return nil, err
	}

	return &sshCommitSignature{
		publicKey:     wire.PublicKey,
		namespace:     wire.Namespace,
		reserved:      wire.Reserved,
		hashAlgorithm: wire.HashAlgorithm,
		signature:     sshSignature,
	}, nil
}

func sshSignedData(namespace, reserved, hashAlgorithm string, payload []byte) ([]byte, error) {
	var digest []byte
	switch hashAlgorithm {
	case "sha256":
		sum := sha256.Sum256(payload)
		digest = sum[:]
	case "sha512":
		sum := sha512.Sum512(payload)
		digest = sum[:]
	default:
		return nil, fmt.Errorf("unsupported SSH signature hash algorithm %q", hashAlgorithm)
	}

	return append([]byte("SSHSIG"), ssh.Marshal(struct {
		Namespace     string
		Reserved      string
		HashAlgorithm string
		Hash          []byte
	}{
		Namespace:     namespace,
		Reserved:      reserved,
		HashAlgorithm: hashAlgorithm,
		Hash:          digest,
	})...), nil
}
