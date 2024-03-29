package controller

import (
	"encoding/json"
	"fmt"

	"dario.cat/mergo"
	jsonpatch "github.com/evanphx/json-patch"
	bananav1alpha1 "github.com/middlewaregruppen/banana-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PatchOption func(*Patcher) ([]byte, error)

type Patcher struct {
	original *runtime.RawExtension
	objs     []*runtime.RawExtension
	opts     []PatchOption
}

type JsonPatch struct {
	Op    string
	Path  string
	Value string
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

// Add adds an object to the patcher
func (p *Patcher) Add(obj *runtime.RawExtension) *Patcher {
	p.objs = append(p.objs, obj)
	return p
}

// Use adds a PatchOption middlware to be used during build
func (p *Patcher) Use(opt PatchOption) *Patcher {
	p.opts = append(p.opts, opt)
	return p
}

// Build merges all objects added by Add() and applies all middlwares. Lastly flattened list of objects & middlwares
// Are merged into the destination object provided by New() returning a final flattend []byte
func (p *Patcher) Build() ([]byte, error) {
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

	// Use an empty runtime.RawExtension if nil
	original := p.original
	if original == nil {
		original = &runtime.RawExtension{Raw: []byte("{}")}
	}

	// Now merge into the original runtime.RawExtension
	m, err := compile(original)
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

	// Apply middleware
	for _, opt := range p.opts {
		b, err = opt(p)
		if err != nil {
			return nil, err
		}
		p.original = &runtime.RawExtension{Raw: b}
	}

	return p.original.Raw, nil
}

// JsonPatch6902 is a PatchOption that performs Json 6902 patch operations on the original object based on the provided patch type
func JsonPatch6902(patches ...*bananav1alpha1.Patch) PatchOption {
	return func(p *Patcher) ([]byte, error) {
		b := p.original.Raw
		for _, patch := range patches {
			patchJSON := []byte(fmt.Sprintf("[{\"op\": \"%s\", \"path\": \"%s\", \"value\": \"%s\"}]", patch.Op, patch.Path, patch.Value))
			jpatch, err := jsonpatch.DecodePatch(patchJSON)
			if err != nil {
				return nil, err
			}
			b, err = jpatch.Apply(b)
			if err != nil {
				return nil, err
			}
		}
		return b, nil
	}
}

// NewPatcherFor returns a new patcher for the given object. Provide optional PatchOptions if additional operations are desired
func NewPatcherFor(obj *runtime.RawExtension, opts ...PatchOption) *Patcher {
	return &Patcher{
		original: obj,
		opts:     opts,
	}
}
