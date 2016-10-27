// Code generated by ""fitask" -type=ElasticIP"; DO NOT EDIT

package awstasks

import (
	"encoding/json"

	"k8s.io/kops/upup/pkg/fi"
)

// ElasticIP

// JSON marshalling boilerplate
type realElasticIP ElasticIP

func (o *ElasticIP) UnmarshalJSON(data []byte) error {
	var jsonName string
	if err := json.Unmarshal(data, &jsonName); err == nil {
		o.Name = &jsonName
		return nil
	}

	var r realElasticIP
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	*o = ElasticIP(r)
	return nil
}

var _ fi.HasName = &ElasticIP{}

func (e *ElasticIP) GetName() *string {
	return e.Name
}

func (e *ElasticIP) SetName(name string) {
	e.Name = &name
}

func (e *ElasticIP) String() string {
	return fi.TaskAsString(e)
}
