package control_plane

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/KeaganFerrao/adaptive-control-plane/internal/models"
)

type ControlPlane struct {
	policies            []*models.Policy
	metricCollector     models.MetricCollector
	actionRunner        models.ActionRunner
	policyEvaluator     models.PolicyEvaluator
	policyCheckInterval time.Duration
}

func NewControlPlane(
	policies []*models.Policy,
	metricCollector models.MetricCollector,
	actionRunner models.ActionRunner,
	policyEvaluator models.PolicyEvaluator,
	policyCheckInterval time.Duration,
) *ControlPlane {
	return &ControlPlane{
		policies:            policies,
		metricCollector:     metricCollector,
		actionRunner:        actionRunner,
		policyEvaluator:     policyEvaluator,
		policyCheckInterval: policyCheckInterval,
	}
}

func (cp *ControlPlane) Start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	for _, policy := range cp.policies {
		wg.Go(func() {
			cp.startPolicyCheck(ctx, policy)
		})
	}

	stop := func() {
		cancel()
		wg.Wait()
	}

	return stop
}

func (cp *ControlPlane) startPolicyCheck(ctx context.Context, policy *models.Policy) {
	logger := log.New(log.Default().Writer(), fmt.Sprintf("[ControlPlane:Policy:%s] ", policy.PolicyName), log.Default().Flags()|log.Lmsgprefix|log.Lmicroseconds)

	defer func() {
		if r := recover(); r != nil {
			logger.Printf(
				"Panic in policy check goroutine: policy=%s panic=%v\n%s\n",
				policy.PolicyName,
				r,
				debug.Stack(),
			)
		}
	}()

	logger.Println("Starting policy check goroutine")
	ticker := time.NewTicker(cp.policyCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Println("Stopping policy check goroutine due to context canceled")
			return

		case <-ticker.C:
			if err := cp.checkPolicy(ctx, policy, logger); err != nil {
				logger.Println("Error in policy check:", err)
			}
		}
	}
}

func (cp *ControlPlane) checkPolicy(ctx context.Context, policy *models.Policy, logger *log.Logger) error {
	logger.Println("Checking policy")

	collect, err := cp.metricCollector.Collect(ctx, *policy.InterestedMetrics)
	if err != nil {
		logger.Println("Error collecting metrics:", err)
		return err
	}

	logger.Println("Collected metrics:", collect)
	policy.ComponentMetricSnapshots = collect

	evaluate, err := cp.policyEvaluator.Evaluate(ctx, policy)
	if err != nil {
		logger.Println("Error evaluating policy:", err)
		return err
	}

	logger.Println("Policy evaluation result:", evaluate)

	if evaluate.TriggerCondition == models.ConditionTrue {
		logger.Println("Trigger condition is currently true")

		policy.RecoveryFirstSeenAt = nil

		// Condition is not yet triggered
		if policy.Status != models.PolicyStatusTriggered {
			logger.Println("Condition is not yet triggered")
			// Condition is seen before
			if policy.ConditionFirstSeenAt != nil {
				logger.Println("Condition is seen before")
				// Its already been "For" duration past since first seen
				if cp.isDifferenceGreaterThan(*policy.ConditionFirstSeenAt, time.Now(), policy.PolicyDefinition.For.Duration) {
					logger.Println("Its already been FOR duration past since first seen trigger condition")

					logger.Println("--------TRIGGERING ACTIONS---------")

					// Need to trigger the actions
					// Update status of policy to triggered

					run, err := cp.actionRunner.Run(ctx, policy.PolicyDefinition.Then)
					if err != nil {
						logger.Println("Error running actions:", err)
						return err
					}

					didAnyActionFail := false
					for _, result := range run {
						if result.Status == models.ActionResultStatusFailure {
							logger.Println("One action resulted in failure so not updating policy status to triggered")
							didAnyActionFail = true
							break
						}
					}

					if !didAnyActionFail {
						policy.Status = models.PolicyStatusTriggered
					}
				} else {
					logger.Println("Not FOR duration past since first seen trigger condition")
					// Not "For" duration past
					// Update status of policy to pending_trigger

					//policy.Status = models.PolicyStatusPendingTrigger
				}
			} else {
				logger.Println("Condition is not seen before")
				// Condition is not seen before
				// Update condition first seen to now()
				// Update RecoveryFirstSeenAt to nil
				// Update status of policy to pending_trigger
				now := time.Now()
				policy.ConditionFirstSeenAt = &now

				//policy.Status = models.PolicyStatusPendingTrigger
			}
		} else {
			logger.Println("Policy status is already triggered, no need to trigger action again")
			// Actions are already triggered
		}
	} else if evaluate.TriggerCondition == models.ConditionFalse {
		logger.Println("Trigger condition is currently false")

		// Do we need to do anything here?
	} else if evaluate.TriggerCondition == models.ConditionUnknown {
		logger.Println("Trigger condition is currently unknown")
	}

	if evaluate.RecoveryCondition == models.ConditionTrue {
		logger.Println("Recovery condition is currently true")

		policy.ConditionFirstSeenAt = nil

		// Recovery is not yet triggered
		if policy.Status != models.PolicyStatusNormal {
			logger.Println("Recovery condition is not yet triggered")
			// Recovery is seen before
			if policy.RecoveryFirstSeenAt != nil {
				logger.Println("Recovery condition is seen before")
				// Its already been "For" duration past since first recovery seen
				if cp.isDifferenceGreaterThan(*policy.RecoveryFirstSeenAt, time.Now(), policy.PolicyDefinition.Recover.For.Duration) {
					logger.Println("Its already been FOR duration past since first recovery seen")

					logger.Println("--------TRIGGERING RECOVERY ACTION---------")
					// Need to trigger recovery actions
					// Update status of policy to normal

					run, err := cp.actionRunner.Run(ctx, policy.PolicyDefinition.Recover.Then)
					if err != nil {
						return err
					}

					didAnyActionFail := false
					for _, result := range run {
						if result.Status == models.ActionResultStatusFailure {
							logger.Println("One recovery action resulted in failure so not updating policy status to triggered")
							didAnyActionFail = true
							break
						}
					}

					if !didAnyActionFail {
						policy.Status = models.PolicyStatusNormal
					}
				} else {
					logger.Println("Not FOR duration past since first recovery seen")
					// Not "For" duration past
					// Update status of policy to pending_recovery
					//policy.Status = models.PolicyStatusPendingRecovery
				}
			} else {
				logger.Println("Recovery condition is seen before")
				// Recovery is not seen before
				// Update recovery first seen to now()
				// Update condition first seen to nil
				// Update status of policy to pending_recovery
				now := time.Now()
				policy.RecoveryFirstSeenAt = &now
				//policy.Status = models.PolicyStatusPendingRecovery
			}
		} else {
			// Condition is already normal, no need to trigger recovery actions
			logger.Println("Policy status is already normal, not need to trigger recovery action")
		}

	} else if evaluate.RecoveryCondition == models.ConditionFalse {
		logger.Println("Recovery condition is currently false")
	} else if evaluate.RecoveryCondition == models.ConditionUnknown {
		logger.Println("Recovery condition is currently unknown")
	}

	return nil
}

func (cp *ControlPlane) isDifferenceGreaterThan(start, end time.Time, max time.Duration) bool {
	return end.Sub(start) > max
}
