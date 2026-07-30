package service

import (
	"context"
	"fmt"
)

var gatewayTimeoutRuntimeSettingKeys = []string{
	SettingKeyOpenAIFirstOutputTimeoutSeconds,
	SettingKeyOpenAIHighEffortFirstOutputTimeoutSeconds,
	SettingKeyOpenAIFirstOutputFailoverEnabled,
	SettingKeyOpenAIFirstOutputInitialAttemptTimeoutSeconds,
	SettingKeyOpenAIFirstOutputMaxSwitches,
	SettingKeyOpenAIFirstOutputPenalizeAccount,
	SettingKeyStreamDataIntervalTimeout,
}

func validateZeroOrRange(name string, value, min, max int) error {
	if value == 0 {
		return nil
	}
	if value < min || value > max {
		return fmt.Errorf("%s must be 0 or between %d and %d", name, min, max)
	}
	return nil
}

func validateRange(name string, value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return nil
}

func normalizeGatewayTimeoutRuntimeSettings(settings *SystemSettings) error {
	if settings == nil {
		return nil
	}
	if err := validateZeroOrRange(SettingKeyOpenAIFirstOutputTimeoutSeconds, settings.OpenAIFirstOutputTimeoutSeconds, 5, 600); err != nil {
		return err
	}
	if err := validateZeroOrRange(SettingKeyOpenAIHighEffortFirstOutputTimeoutSeconds, settings.OpenAIHighEffortFirstOutputTimeoutSeconds, 5, 1800); err != nil {
		return err
	}
	if err := validateRange(SettingKeyOpenAIFirstOutputInitialAttemptTimeoutSeconds, settings.OpenAIFirstOutputInitialAttemptTimeoutSeconds, 0, 600); err != nil {
		return err
	}
	if err := validateRange(SettingKeyOpenAIFirstOutputMaxSwitches, settings.OpenAIFirstOutputMaxSwitches, 0, 5); err != nil {
		return err
	}
	if err := validateZeroOrRange(SettingKeyStreamDataIntervalTimeout, settings.StreamDataIntervalTimeout, 30, 300); err != nil {
		return err
	}
	return nil
}

func (s *SettingService) applyGatewayTimeoutRuntimeSettings(settings *SystemSettings) {
	if s == nil || s.cfg == nil || settings == nil {
		return
	}
	s.cfg.Gateway.OpenAIFirstOutputTimeoutSeconds = settings.OpenAIFirstOutputTimeoutSeconds
	s.cfg.Gateway.OpenAIHighEffortFirstOutputTimeoutSeconds = settings.OpenAIHighEffortFirstOutputTimeoutSeconds
	s.cfg.Gateway.OpenAIFirstOutputFailoverEnabled = settings.OpenAIFirstOutputFailoverEnabled
	s.cfg.Gateway.OpenAIFirstOutputInitialAttemptTimeoutSeconds = settings.OpenAIFirstOutputInitialAttemptTimeoutSeconds
	s.cfg.Gateway.OpenAIFirstOutputMaxSwitches = settings.OpenAIFirstOutputMaxSwitches
	s.cfg.Gateway.OpenAIFirstOutputPenalizeAccount = settings.OpenAIFirstOutputPenalizeAccount
	s.cfg.Gateway.StreamDataIntervalTimeout = settings.StreamDataIntervalTimeout
}

// LoadGatewayTimeoutRuntimeSettings overlays DB-backed gateway timeout knobs onto
// the in-process config during startup. Existing hot paths already read cfg, so a
// page save also applies immediately via refreshCachedSettings.
func (s *SettingService) LoadGatewayTimeoutRuntimeSettings(ctx context.Context) error {
	if s == nil || s.settingRepo == nil || s.cfg == nil {
		return nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, gatewayTimeoutRuntimeSettingKeys)
	if err != nil {
		return fmt.Errorf("get gateway timeout runtime settings: %w", err)
	}
	settings := s.parseSettings(values)
	if err := normalizeGatewayTimeoutRuntimeSettings(settings); err != nil {
		return err
	}
	s.applyGatewayTimeoutRuntimeSettings(settings)
	return nil
}
