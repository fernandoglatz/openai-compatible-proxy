package service

import (
	"context"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"fmt"
	"sync"
	"time"
)

type IdleMonitor struct {
	lastActivity time.Time
	mu           sync.RWMutex
	ticker       *time.Ticker
	stopChan     chan bool
	isRunning    bool
	suspendSent  bool // Track if suspend message has been sent
}

var monitor *IdleMonitor
var monitorOnce sync.Once

// GetIdleMonitor returns the singleton instance of IdleMonitor
func GetIdleMonitor() *IdleMonitor {
	monitorOnce.Do(func() {
		monitor = &IdleMonitor{
			lastActivity: time.Now(),
			stopChan:     make(chan bool),
			isRunning:    false,
		}
	})
	return monitor
}

// Start begins monitoring for idle timeout
func (im *IdleMonitor) Start(ctx context.Context) {
	mqttConfig := config.ApplicationConfig.MQTT

	if !mqttConfig.Enabled {
		log.Info(ctx).Msg("Idle monitor not started - MQTT is disabled")
		return
	}

	im.mu.Lock()
	if im.isRunning {
		im.mu.Unlock()
		log.Warn(ctx).Msg("Idle monitor is already running")
		return
	}
	im.isRunning = true
	im.mu.Unlock()

	// Check every minute
	im.ticker = time.NewTicker(1 * time.Minute)

	log.Info(ctx).Msg(fmt.Sprintf("Idle monitor started - will send suspend message after %v of inactivity", mqttConfig.Idle.Timeout))

	go func() {
		for {
			select {
			case <-im.ticker.C:
				im.checkIdle(ctx)
			case <-im.stopChan:
				im.ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the idle monitor
func (im *IdleMonitor) Stop(ctx context.Context) {
	im.mu.Lock()
	defer im.mu.Unlock()

	if !im.isRunning {
		return
	}

	log.Info(ctx).Msg("Stopping idle monitor")
	im.stopChan <- true
	im.isRunning = false
}

// RecordActivity updates the last activity timestamp and resets suspend flag
func (im *IdleMonitor) RecordActivity() {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.lastActivity = time.Now()
	im.suspendSent = false // Reset the suspend flag when there's activity
}

// checkIdle checks if the system has been idle for too long
func (im *IdleMonitor) checkIdle(ctx context.Context) {
	im.mu.RLock()
	lastActivity := im.lastActivity
	suspendSent := im.suspendSent
	im.mu.RUnlock()

	mqttConfig := config.ApplicationConfig.MQTT
	idleDuration := time.Since(lastActivity)

	if idleDuration >= mqttConfig.Idle.Timeout && !suspendSent {
		log.Info(ctx).Msg(fmt.Sprintf("System has been idle for %v (threshold: %v) - sending suspend message",
			idleDuration.Round(time.Second), mqttConfig.Idle.Timeout))

		err := utils.PublishMessage(ctx, mqttConfig.Idle.Message)
		if err != nil {
			log.Error(ctx).Msg(fmt.Sprintf("Failed to publish idle suspend message: %v", err))
		} else {
			// Mark suspend message as sent - won't send again until activity is detected
			im.mu.Lock()
			im.suspendSent = true
			im.mu.Unlock()
			log.Info(ctx).Msg("Suspend message sent - will not send again until system becomes active")
		}
	} else if idleDuration < mqttConfig.Idle.Timeout {
		timeUntilIdle := mqttConfig.Idle.Timeout - idleDuration
		log.Debug(ctx).Msg(fmt.Sprintf("System is active - time until idle: %v", timeUntilIdle.Round(time.Second)))
	}
}

// GetIdleDuration returns how long the system has been idle
func (im *IdleMonitor) GetIdleDuration() time.Duration {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return time.Since(im.lastActivity)
}
