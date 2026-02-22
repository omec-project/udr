// SPDX-FileCopyrightText: 2025 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
/*
 * NRF Registration Unit Testcases
 *
 */
package nfregistration

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/udr/consumer"
)

func TestNfRegistrationService_WhenEmptyConfig_ThenDeregisterNFAndStopTimer(t *testing.T) {
	isDeregisterNFCalled := false
	testCases := []struct {
		name                         string
		sendDeregisterNFInstanceMock func() error
	}{
		{
			name: "Success",
			sendDeregisterNFInstanceMock: func() error {
				isDeregisterNFCalled = true
				return nil
			},
		},
		{
			name: "ErrorInDeregisterNFInstance",
			sendDeregisterNFInstanceMock: func() error {
				isDeregisterNFCalled = true
				return errors.New("mock error")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keepAliveTimer = time.NewTimer(60 * time.Second)
			isRegisterNFCalled := false
			isDeregisterNFCalled = false
			originalDeregisterNF := consumer.SendDeregisterNFInstance
			originalRegisterNF := registerNF

			consumer.SendDeregisterNFInstance = tc.sendDeregisterNFInstanceMock
			registerNF = func(ctx context.Context, newPlmnConfig []models.PlmnId) {
				isRegisterNFCalled = true
			}

			ch := make(chan []models.PlmnId, 1)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				StartNfRegistrationService(ctx, ch)
				close(done)
			}()
			defer func() {
				cancel()
				<-done
				consumer.SendDeregisterNFInstance = originalDeregisterNF
				registerNF = originalRegisterNF
				keepAliveTimerMutex.Lock()
				if keepAliveTimer != nil {
					keepAliveTimer.Stop()
					keepAliveTimer = nil
				}
				keepAliveTimerMutex.Unlock()
			}()

			ch <- []models.PlmnId{}
			time.Sleep(100 * time.Millisecond)
			cancel()
			<-done

			if keepAliveTimer != nil {
				t.Errorf("expected keepAliveTimer to be nil after stopKeepAliveTimer")
			}
			if !isDeregisterNFCalled {
				t.Errorf("expected SendDeregisterNFInstance to be called")
			}
			if isRegisterNFCalled {
				t.Errorf("expected registerNF not to be called")
			}
		})
	}
}

func TestNfRegistrationService_WhenConfigChanged_ThenRegisterNFSuccessAndStartTimer(t *testing.T) {
	keepAliveTimer = nil
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	originalRegisterNF := registerNF

	registrations := []models.PlmnId{}
	consumer.SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (models.NfProfile, string, error) {
		profile := models.NfProfile{HeartBeatTimer: 60}
		registrations = append(registrations, plmnConfig...)
		return profile, "", nil
	}

	registerNFDone := make(chan struct{}, 1)
	registerNF = func(ctx context.Context, plmnConfig []models.PlmnId) {
		originalRegisterNF(ctx, plmnConfig)
		registerNFDone <- struct{}{}
	}

	ch := make(chan []models.PlmnId, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		StartNfRegistrationService(ctx, ch)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
		registerNF = originalRegisterNF
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		keepAliveTimerMutex.Lock()
		if keepAliveTimer != nil {
			keepAliveTimer.Stop()
			keepAliveTimer = nil
		}
		keepAliveTimerMutex.Unlock()
	}()

	newConfig := []models.PlmnId{{Mcc: "001", Mnc: "01"}}
	ch <- newConfig
	<-registerNFDone

	if keepAliveTimer == nil {
		t.Error("expected keepAliveTimer to be initialized by startKeepAliveTimer")
	}
	if !reflect.DeepEqual(registrations, newConfig) {
		t.Errorf("Expected %+v config, received %+v", newConfig, registrations)
	}
}

func TestNfRegistrationService_ConfigChanged_RetryIfRegisterNFFails(t *testing.T) {
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	originalRegisterNF := registerNF

	called := 0
	consumer.SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (models.NfProfile, string, error) {
		profile := models.NfProfile{HeartBeatTimer: 60}
		called++
		return profile, "", errors.New("mock error")
	}

	registerNFDone := make(chan struct{}, 1)
	registerNF = func(ctx context.Context, plmnConfig []models.PlmnId) {
		originalRegisterNF(ctx, plmnConfig)
		registerNFDone <- struct{}{}
	}

	ch := make(chan []models.PlmnId, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		StartNfRegistrationService(ctx, ch)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
		registerNF = originalRegisterNF
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		keepAliveTimerMutex.Lock()
		if keepAliveTimer != nil {
			keepAliveTimer.Stop()
			keepAliveTimer = nil
		}
		keepAliveTimerMutex.Unlock()
	}()

	ch <- []models.PlmnId{{Mcc: "001", Mnc: "01"}}
	time.Sleep(2 * retryTime)
	cancel()
	<-done
	<-registerNFDone

	if called < 2 {
		t.Error("Expected to retry register to NRF")
	}
	t.Logf("Tried %v times", called)
}

func TestNfRegistrationService_WhenConfigChanged_ThenPreviousRegistrationIsCancelled(t *testing.T) {
	originalRegisterNf := registerNF

	registered := make(chan struct{}, 2)
	var registrations []struct {
		ctx    context.Context
		config []models.PlmnId
	}
	registerNF = func(registerCtx context.Context, newPlmnConfig []models.PlmnId) {
		registrations = append(registrations, struct {
			ctx    context.Context
			config []models.PlmnId
		}{registerCtx, newPlmnConfig})
		registered <- struct{}{}
		<-registerCtx.Done() // Wait until registration is cancelled
	}

	ch := make(chan []models.PlmnId, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		StartNfRegistrationService(ctx, ch)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
		registerNF = originalRegisterNf
		keepAliveTimerMutex.Lock()
		if keepAliveTimer != nil {
			keepAliveTimer.Stop()
			keepAliveTimer = nil
		}
		keepAliveTimerMutex.Unlock()
	}()

	firstConfig := []models.PlmnId{{Mcc: "001", Mnc: "01"}}
	ch <- firstConfig
	<-registered
	if len(registrations) != 1 {
		t.Error("expected one registration to the NRF")
	}

	secondConfig := []models.PlmnId{{Mcc: "002", Mnc: "02"}}
	ch <- secondConfig
	<-registered
	if len(registrations) != 2 {
		t.Error("expected 2 registrations to the NRF")
	}

	select {
	case <-registrations[0].ctx.Done():
		// expected
	default:
		t.Error("expected first registration context to be cancelled")
	}

	select {
	case <-registrations[1].ctx.Done():
		t.Error("second registration context should not be cancelled")
	default:
		// expected
	}

	if !reflect.DeepEqual(registrations[0].config, firstConfig) {
		t.Errorf("Expected %+v config, received %+v", firstConfig, registrations)
	}
	if !reflect.DeepEqual(registrations[1].config, secondConfig) {
		t.Errorf("Expected %+v config, received %+v", secondConfig, registrations)
	}
}

func TestHeartbeatNF_Success(t *testing.T) {
	keepAliveTimer = time.NewTimer(60 * time.Second)
	calledRegister := false
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	originalSendUpdateNFInstance := consumer.SendUpdateNFInstance
	defer func() {
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		consumer.SendUpdateNFInstance = originalSendUpdateNFInstance
		if keepAliveTimer != nil {
			keepAliveTimer.Stop()
		}
	}()

	consumer.SendUpdateNFInstance = func(patchItem []models.PatchItem) (models.NfProfile, *models.ProblemDetails, error) {
		return models.NfProfile{}, nil, nil
	}
	consumer.SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (models.NfProfile, string, error) {
		calledRegister = true
		profile := models.NfProfile{HeartBeatTimer: 60}
		return profile, "", nil
	}
	plmnConfig := []models.PlmnId{}
	heartbeatNF(plmnConfig)

	if calledRegister {
		t.Errorf("expected registerNF to be called on error")
	}
	if keepAliveTimer == nil {
		t.Error("expected keepAliveTimer to be initialized by startKeepAliveTimer")
	}
}

func TestHeartbeatNF_WhenNfUpdateFails_ThenNfRegistersIsCalled(t *testing.T) {
	keepAliveTimer = time.NewTimer(60 * time.Second)
	calledRegister := false
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	originalSendUpdateNFInstance := consumer.SendUpdateNFInstance
	defer func() {
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		consumer.SendUpdateNFInstance = originalSendUpdateNFInstance
		if keepAliveTimer != nil {
			keepAliveTimer.Stop()
		}
	}()

	consumer.SendUpdateNFInstance = func(patchItem []models.PatchItem) (models.NfProfile, *models.ProblemDetails, error) {
		return models.NfProfile{}, nil, errors.New("mock error")
	}

	consumer.SendRegisterNFInstance = func(plmnConfig []models.PlmnId) (models.NfProfile, string, error) {
		profile := models.NfProfile{HeartBeatTimer: 60}
		calledRegister = true
		return profile, "", nil
	}

	plmnConfig := []models.PlmnId{}
	heartbeatNF(plmnConfig)

	if !calledRegister {
		t.Errorf("expected registerNF to be called on error")
	}
	if keepAliveTimer == nil {
		t.Error("expected keepAliveTimer to be initialized by startKeepAliveTimer")
	}
}

func TestStartKeepAliveTimer_UsesProfileTimerOnlyWhenGreaterThanZero(t *testing.T) {
	testCases := []struct {
		name             string
		profileTime      int32
		expectedDuration time.Duration
	}{
		{
			name:             "Profile heartbeat time is zero, use default time",
			profileTime:      0,
			expectedDuration: 60 * time.Second,
		},
		{
			name:             "Profile heartbeat time is smaller than zero, use default time",
			profileTime:      -5,
			expectedDuration: 60 * time.Second,
		},
		{
			name:             "Profile heartbeat time is greater than zero, use profile time",
			profileTime:      15,
			expectedDuration: 15 * time.Second,
		},
		{
			name:             "Profile heartbeat time is greater than default time, use profile time",
			profileTime:      90,
			expectedDuration: 90 * time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keepAliveTimer = time.NewTimer(25 * time.Second)
			defer func() {
				if keepAliveTimer != nil {
					keepAliveTimer.Stop()
				}
			}()
			var capturedDuration time.Duration

			afterFunc = func(d time.Duration, _ func()) *time.Timer {
				capturedDuration = d
				return time.NewTimer(25 * time.Second)
			}
			defer func() { afterFunc = time.AfterFunc }()

			startKeepAliveTimer(tc.profileTime, nil)
			if tc.expectedDuration != capturedDuration {
				t.Errorf("Expected %v duration, got %v", tc.expectedDuration, capturedDuration)
			}
		})
	}
}
