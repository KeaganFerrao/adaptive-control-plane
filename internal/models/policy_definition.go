package models

import (
	"time"
)

type PolicyDefinition struct {
	Name    string         `yaml:"name"`
	When    Expression     `yaml:"when"`
	For     Duration       `yaml:"for"`
	Then    []PolicyAction `yaml:"then"`
	Recover Recovery       `yaml:"recover"`
}

type PolicyAction struct {
	Action     ActionName     `yaml:"action"`
	Parameters map[string]any `yaml:"parameters"`
	Handler    ActionHandler  `yaml:"-"`
}

type Recovery struct {
	When Expression     `yaml:"when"`
	For  Duration       `yaml:"for"`
	Then []PolicyAction `yaml:"then"`
}

type Expression struct {
	And       []Expression `yaml:"and,omitempty"`
	Or        []Expression `yaml:"or,omitempty"`
	Condition *Condition   `yaml:"condition,omitempty"`
}

type Condition struct {
	Metric    string   `yaml:"metric,omitempty"`
	Operator  Operator `yaml:"operator,omitempty"`
	Threshold float64  `yaml:"threshold,omitempty"`

	// Implement in v2
	ThresholdType ThresholdType `yaml:"threshold_type,omitempty"`
	Scope         *Scope        `yaml:"scope,omitempty"`
	Aggregation   *Aggregation  `yaml:"aggregation,omitempty"`
}

type Aggregation struct {
	Type AggregationType `yaml:"type"`
}

type AggregationType string

const (
	AggregationAvg AggregationType = "avg"
	AggregationMin AggregationType = "min"
	AggregationMax AggregationType = "max"
	AggregationSum AggregationType = "sum"
)

type Scope struct {
	Component string `yaml:"component,omitempty"`
	Instance  string `yaml:"instance,omitempty"`
}

type Operator string

const (
	OperatorGreaterThan    Operator = ">"
	OperatorGreaterOrEqual Operator = ">="
	OperatorLessThan       Operator = "<"
	OperatorLessOrEqual    Operator = "<="
	OperatorEqual          Operator = "=="
	OperatorNotEqual       Operator = "!="
)

type ThresholdType string

const (
	ThresholdAbsolute   ThresholdType = "absolute"
	ThresholdPercentage ThresholdType = "percentage"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var value string

	if err := unmarshal(&value); err != nil {
		return err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}

	d.Duration = duration
	return nil
}
