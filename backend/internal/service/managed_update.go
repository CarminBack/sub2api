package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrManagedUpdateQueued     = errors.New("managed update queued")
	ErrManagedUpdateInProgress = infraerrors.Conflict("UPDATE_IN_PROGRESS", "a managed update is already in progress")
)

var token3VersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+-token3\.\d+$`)

const (
	managedUpdateDirName     = "token3-update"
	managedUpdateRequestFile = "request.json"
	managedUpdateStatusFile  = "status.json"
)

type ManagedUpdateRequest struct {
	CurrentVersion string    `json:"current_version"`
	TargetVersion  string    `json:"target_version"`
	RequestedAt    time.Time `json:"requested_at"`
}

type ManagedUpdateStatus struct {
	State          string     `json:"state"`
	Message        string     `json:"message,omitempty"`
	CurrentVersion string     `json:"current_version,omitempty"`
	TargetVersion  string     `json:"target_version,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

func isToken3Version(version string) bool {
	return token3VersionPattern.MatchString(strings.TrimPrefix(strings.TrimSpace(version), "v"))
}

func (s *UpdateService) queueManagedUpdate(targetVersion string) error {
	dir, err := managedUpdateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create managed update directory: %w", err)
	}

	requestPath := filepath.Join(dir, managedUpdateRequestFile)
	if _, err := os.Stat(requestPath); err == nil {
		return ErrManagedUpdateInProgress
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check managed update request: %w", err)
	}

	status, err := s.GetManagedUpdateStatus()
	if err != nil {
		return err
	}
	if isManagedUpdateActive(status.State) {
		return ErrManagedUpdateInProgress
	}

	request := ManagedUpdateRequest{
		CurrentVersion: s.currentVersion,
		TargetVersion:  strings.TrimPrefix(strings.TrimSpace(targetVersion), "v"),
		RequestedAt:    time.Now().UTC(),
	}
	return writeJSONAtomic(requestPath, request, 0644)
}

func (s *UpdateService) GetManagedUpdateStatus() (*ManagedUpdateStatus, error) {
	dir, err := managedUpdateDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, managedUpdateStatusFile))
	if errors.Is(err, os.ErrNotExist) {
		return &ManagedUpdateStatus{State: "idle", CurrentVersion: s.currentVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read managed update status: %w", err)
	}

	var status ManagedUpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("decode managed update status: %w", err)
	}
	if strings.TrimSpace(status.State) == "" {
		return nil, fmt.Errorf("decode managed update status: state is required")
	}
	return &status, nil
}

func managedUpdateDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("TOKEN3_UPDATE_DIR")); dir != "" {
		return dir, nil
	}
	if dataDir := strings.TrimSpace(os.Getenv("DATA_DIR")); dataDir != "" {
		return filepath.Join(dataDir, managedUpdateDirName), nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve managed update directory: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), "data", managedUpdateDirName), nil
}

func isManagedUpdateActive(state string) bool {
	switch state {
	case "queued", "syncing", "building", "deploying":
		return true
	default:
		return false
	}
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode managed update request: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".managed-update-*")
	if err != nil {
		return fmt.Errorf("create managed update request: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set managed update request permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write managed update request: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync managed update request: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close managed update request: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish managed update request: %w", err)
	}
	return nil
}
