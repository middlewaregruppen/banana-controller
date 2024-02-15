package controller

import (
	"encoding/json"

	"dario.cat/mergo"
	"k8s.io/apimachinery/pkg/runtime"
)

type PatchOption func(*Patcher)

type Patcher struct {
	original *runtime.RawExtension
	objs     []*runtime.RawExtension
}

func (p *Patcher) Add(obj *runtime.RawExtension) *Patcher {
	p.objs = append(p.objs, obj)
	return p
}

func (p *Patcher) Build() (*runtime.RawExtension, error) {
	dst := map[string]interface{}{}

	// Merge all objs in the array into one runtime.RawExtension
	for _, o := range p.objs {
		m, err := compile(o)
		if err != nil {
			return nil, err
		}
		err = merge(dst, m)
		if err != nil {
			return nil, err
		}
	}

	// Now merge into the original runtime.RawExtension
	m, err := compile(p.original)
	if err != nil {
		return nil, err
	}

	err = merge(m, dst)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(&m)
	if err != nil {
		return nil, err
	}

	p.original = &runtime.RawExtension{Raw: b}
	return p.original, nil
}

func merge(dst, src map[string]interface{}) error {
	return mergo.Merge(&dst, &src)
}

func compile(obj *runtime.RawExtension) (map[string]interface{}, error) {
	var s map[string]interface{}

	b, err := json.Marshal(&obj)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func WithoutValues() PatchOption {
	return func(p *Patcher) {
		// TODO:
	}
}

func NewPatcherFor(obj *runtime.RawExtension) *Patcher {
	return &Patcher{
		original: obj,
	}
}
