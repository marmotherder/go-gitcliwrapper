package gitcliwrapper

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marmotherder/go-cmdwrapper"
)

type Logger interface {
	Debug(args ...any)
	Debugf(template string, args ...any)
	Infof(template string, args ...any)
	Warnf(template string, args ...any)
	Error(args ...any)
	Errorf(template string, args ...any)
}

func NewGitCLIWrapper(workingDirectory string, logger Logger, remote ...string) *GitCLIWrapper {
	git := &GitCLIWrapper{
		logger: logger,
		cmd: cmdwrapper.CMDWrapper{
			Dir:    workingDirectory,
			Logger: logger,
		},
	}

	if len(remote) > 0 {
		git.remote = remote[0]
	}

	return git
}

const (
	gitCmd          = "git"
	nonZeroCodeText = "command returned a non zero code"
)

type GitCLIWrapper struct {
	remote string
	logger Logger
	cmd    cmdwrapper.CMDWrapper
}

func nonZeroCode(text string) error {
	return fmt.Errorf("%s %s %s", gitCmd, text, nonZeroCodeText)
}

func (git *GitCLIWrapper) GetRemote() error {
	git.logger.Debug("looking up git remote")
	remote, code, err := git.cmd.RunCommand(gitCmd, "remote")
	if err != nil {
		git.logger.Error("failed to lookup git remote")
		return err
	}
	if code != nil && *code != 0 {
		return nonZeroCode("remote")
	}
	if remote == nil {
		return errors.New("failed to find a git remote")
	}

	remoteString := strings.TrimSpace(*remote)
	multipleRemotes := strings.Split(remoteString, "\n")

	if len(multipleRemotes) <= 1 {
		git.remote = remoteString
		return nil
	}

	remoteString = multipleRemotes[len(multipleRemotes)-1]
	git.logger.Warnf("multiple remotes were found, using the last one set '%s'", remoteString)

	git.remote = remoteString
	return nil
}

func (git GitCLIWrapper) GetLastCommitOnRef(ref string) (*string, error) {
	git.logger.Debugf("get most recent commit for reference %s on remote %s", ref, git.remote)
	stdOut, code, err := git.cmd.RunCommand(gitCmd, "rev-list", "-n", "1", ref)
	if code != nil && *code != 0 {
		return nil, nonZeroCode("rev-list")
	}
	if err != nil {
		git.logger.Infof("failed to get commit for reference %s on remote %s", ref, git.remote)
		return nil, err
	}
	if stdOut != nil {
		return stdOut, nil
	}

	return nil, errors.New("failed to get commit on reference")
}

func (git GitCLIWrapper) Fetch() error {
	git.logger.Debugf("running git fetch against remote %s", git.remote)
	_, code, err := git.cmd.RunCommand(gitCmd, "fetch", git.remote)
	if code != nil && *code != 0 {
		return nonZeroCode("fetch")
	}
	return err
}

func (git GitCLIWrapper) ListRemoteRefs(refType string) ([]string, error) {
	git.logger.Infof("attempting to get a list of remote %s in git from %s", refType, git.remote)
	remoteRefsResponse, code, err := git.cmd.RunCommand(gitCmd, "ls-remote", "--"+refType, git.remote)
	if err != nil {
		git.logger.Error("failed to lookup from remote")
		return nil, err
	}
	if code != nil && *code != 0 {
		return nil, nonZeroCode("ls-remote")
	}
	if remoteRefsResponse == nil {
		return nil, fmt.Errorf("failed to find any branches against remote %s", git.remote)
	}

	var remoteRefs []string
	for _, remoteRef := range strings.Split(*remoteRefsResponse, "\n") {
		splitRemoteRef := strings.Split(remoteRef, "refs/"+refType+"/")
		if len(splitRemoteRef) != 2 {
			git.logger.Warnf("attempted to parse a reference of unexpected format: %s", remoteRef)
			continue
		}
		remoteRefs = append(remoteRefs, splitRemoteRef[1])
	}

	return remoteRefs, nil
}

func (git GitCLIWrapper) ListCommits(commitRange ...string) ([]string, error) {
	git.logger.Debug("looking up git commits")
	commitRange = append(commitRange, git.remote)
	stdOut, code, err := git.cmd.RunCommand(gitCmd, append([]string{"log", `--pretty=format:"%H"`}, commitRange...)...)
	if err != nil {
		git.logger.Error("failed to run git log")
		return nil, err
	}
	if code != nil && *code != 0 {
		return nil, nonZeroCode("log")
	}

	gitCommits := []string{}
	commitLines := strings.Split(*stdOut, "\n")
	for _, commitLine := range commitLines {
		git.logger.Debugf("processing commit: %s", commitLine)
		if commitLine != "" {
			gitCommits = append(gitCommits, strings.ReplaceAll(commitLine, "\"", ""))
		}
	}

	return gitCommits, nil
}

func (git GitCLIWrapper) GetCurrentBranch() (*string, error) {
	git.logger.Debug("getting the current branch")
	stdOut, code, err := git.cmd.RunCommand(gitCmd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		git.logger.Error("failed to get the current git branch")
		return nil, err
	}
	if code != nil && *code != 0 {
		return nil, nonZeroCode("rev-parse")
	}

	return stdOut, nil
}

func (git GitCLIWrapper) GetCommitMessageBody(hash string) (*string, error) {
	git.logger.Debugf("getting the commit message for %s", hash)
	stdOut, code, err := git.cmd.RunCommand(gitCmd, "log", "--format=%B", "-n", "1", hash)
	if err != nil {
		git.logger.Errorf("failed to get the commit message for %s", hash)
		return nil, err
	}
	if code != nil && *code != 0 {
		return nil, nonZeroCode("log")
	}

	return stdOut, nil
}

func (git GitCLIWrapper) ForcePushHashToRef(hash, ref, refType string) error {
	git.logger.Debugf("going to try to push %s to %s on remote %s", hash, ref, git.remote)
	_, code, err := git.cmd.RunCommand(gitCmd, "push", "-f", git.remote, fmt.Sprintf("%s:refs/%s/%s", hash, refType, ref))
	if err != nil {
		git.logger.Errorf("failed to force push to git branch %s on remote %s", ref, git.remote)
		return err
	}
	if code != nil && *code != 0 {
		return nonZeroCode("push")
	}

	return nil
}
