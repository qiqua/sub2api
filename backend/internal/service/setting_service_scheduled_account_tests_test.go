//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type scheduledAccountTestsRepoStub struct {
	values map[string]string
	err    error
	calls  int
}

func (s *scheduledAccountTestsRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *scheduledAccountTestsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *scheduledAccountTestsRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *scheduledAccountTestsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *scheduledAccountTestsRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *scheduledAccountTestsRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *scheduledAccountTestsRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func resetScheduledAccountTestsCache(t *testing.T) {
	t.Helper()

	scheduledAccountTestsCache.Store((*cachedScheduledAccountTests)(nil))
	t.Cleanup(func() {
		scheduledAccountTestsCache.Store((*cachedScheduledAccountTests)(nil))
	})
}

func TestIsScheduledAccountTestsEnabled_DefaultsFalseWhenMissing(t *testing.T) {
	resetScheduledAccountTestsCache(t)

	repo := &scheduledAccountTestsRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.False(t, svc.IsScheduledAccountTestsEnabled(context.Background()))
	require.Equal(t, 1, repo.calls)
}

func TestIsScheduledAccountTestsEnabled_ReturnsTrueOnlyForTrue(t *testing.T) {
	resetScheduledAccountTestsCache(t)

	repo := &scheduledAccountTestsRepoStub{
		values: map[string]string{SettingKeyScheduledAccountTestsEnabled: "true"},
	}
	svc := NewSettingService(repo, &config.Config{})

	require.True(t, svc.IsScheduledAccountTestsEnabled(context.Background()))
	require.Equal(t, 1, repo.calls)
}

func TestIsScheduledAccountTestsEnabled_FailsClosedOnDBError(t *testing.T) {
	resetScheduledAccountTestsCache(t)

	repo := &scheduledAccountTestsRepoStub{err: errors.New("db down")}
	svc := NewSettingService(repo, &config.Config{})

	require.False(t, svc.IsScheduledAccountTestsEnabled(context.Background()))
	require.Equal(t, 1, repo.calls)
}

func TestUpdateSettings_PersistsScheduledAccountTestsSwitch(t *testing.T) {
	resetScheduledAccountTestsCache(t)

	repo := &scheduledAccountTestsRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		ScheduledAccountTestsEnabled: true,
	})

	require.NoError(t, err)
	require.Equal(t, "true", repo.values[SettingKeyScheduledAccountTestsEnabled])
	require.True(t, svc.IsScheduledAccountTestsEnabled(context.Background()))
}
