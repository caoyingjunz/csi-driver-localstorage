package imageset

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	kc "github.com/ericchiang/k8s"
	"github.com/ericchiang/k8s/apis/extensions/v1beta1"
	"github.com/juju/errgo"
)

const (
	defaultAnnotationKey = "pixiu-lock"
	defaultTTL           = time.Minute
)

var (
	maskAny            = errgo.MaskFunc(errgo.Any)
	AlreadyLockedError = errgo.New("already locked")
	NotLockedByMeError = errgo.New("not locked by me")
)

// KubeLock is used to provide a distributed lock using Kubernetes annotation data.
// It works by writing data into a specific annotation key.
// Other instance trying to write into the same annotation key will be refused because a resource version is used.
type PixiuLock interface {
	// Acquire tries to acquire the lock.
	// If the lock is already held by us, the lock will be updated.
	// If successfull it returns nil, otherwise it returns an error.
	// Note that Acquire will not renew the lock. To do that, call Acquire every ttl/2.
	Acquire() error

	// Release tries to release the lock.
	// If the lock is already held by us, the lock will be released.
	// If successfull it returns nil, otherwise it returns an error.
	Release() error
}

// NewKubeLock creates a new KubeLock.
// The lock will not be aquired.
func NewKubeLock(annotationKey, ownerID string, ttl time.Duration, metaGet MetaGetter, metaUpdate MetaUpdater) (PixiuLock, error) {
	if annotationKey == "" {
		annotationKey = defaultAnnotationKey
	}
	if ownerID == "" {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			return nil, maskAny(err)
		}
		ownerID = base64.StdEncoding.EncodeToString(id)
	}
	if ttl == 0 {
		ttl = defaultTTL
	}
	if metaGet == nil {
		return nil, maskAny(fmt.Errorf("metaGet cannot be nil"))
	}
	if metaUpdate == nil {
		return nil, maskAny(fmt.Errorf("metaUpdate cannot be nil"))
	}
	return &pixiuLock{
		annotationKey: annotationKey,
		ownerID:       ownerID,
		ttl:           ttl,
		getMeta:       metaGet,
		updateMeta:    metaUpdate,
	}, nil
}

type pixiuLock struct {
	annotationKey string
	ownerID       string
	ttl           time.Duration
	getMeta       MetaGetter
	updateMeta    MetaUpdater
}

type LockData struct {
	Owner     string    `json:"owner"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MetaGetter func() (annotations map[string]string, resourceVersion string, extra interface{}, err error)
type MetaUpdater func(annotations map[string]string, resourceVersion string, extra interface{}) error

// Acquire tries to acquire the lock.
// If the lock is already held by us, the lock will be updated.
// If successfull it returns nil, otherwise it returns an error.
func (l *pixiuLock) Acquire() error {
	// Get current state
	ann, rv, extra, err := l.getMeta()
	if err != nil {
		return maskAny(err)
	}

	// Get lock data
	if ann == nil {
		ann = make(map[string]string)
	}
	if lockDataRaw, ok := ann[l.annotationKey]; ok && lockDataRaw != "" {
		var lockData LockData
		if err := json.Unmarshal([]byte(lockDataRaw), &lockData); err != nil {
			return maskAny(err)
		}
		if lockData.Owner != l.ownerID {
			// Lock is owned by someone else
			if time.Now().Before(lockData.ExpiresAt) {
				// Lock is held and not expired
				return maskAny(errgo.WithCausef(nil, AlreadyLockedError, "locked by %s", lockData.Owner))
			}
		}
	}

	// Try to lock it now
	expiredAt := time.Now().Add(l.ttl)
	lockDataRaw, err := json.Marshal(LockData{Owner: l.ownerID, ExpiresAt: expiredAt})
	if err != nil {
		return maskAny(err)
	}
	ann[l.annotationKey] = string(lockDataRaw)
	if err := l.updateMeta(ann, rv, extra); err != nil {
		return maskAny(err)
	}
	// Update successfull, we've acquired the lock
	return nil
}

// Release tries to release the lock.
// If the lock is already held by us, the lock will be released.
// If successfull it returns nil, otherwise it returns an error.
func (l *pixiuLock) Release() error {
	// Get current state
	ann, rv, extra, err := l.getMeta()
	if err != nil {
		return maskAny(err)
	}
	// Get lock data
	if ann == nil {
		ann = make(map[string]string)
	}
	if lockDataRaw, ok := ann[l.annotationKey]; ok && lockDataRaw != "" {
		var lockData LockData
		if err := json.Unmarshal([]byte(lockDataRaw), &lockData); err != nil {
			return maskAny(err)
		}
		if lockData.Owner != l.ownerID {
			// Lock is owned by someone else
			return maskAny(errgo.WithCausef(nil, NotLockedByMeError, "locked by %s", lockData.Owner))
		}
	} else if ok && lockDataRaw == "" {
		// Lock is not locked, we consider that a successfull release also.
		return nil
	}

	// Try to release lock it now
	ann[l.annotationKey] = ""
	if err := l.updateMeta(ann, rv, extra); err != nil {
		return maskAny(err)
	}
	// Update successfull, we've released the lock
	return nil
}

// NewDaemonSetLock creates a lock that uses a DaemonSet to hold the lock data.
func NewDaemonSetLock(namespace, name string, c *kc.Client, annotationKey, ownerID string, ttl time.Duration) (PixiuLock, error) {
	helper := &k8sHelper{
		name:      name,
		namespace: namespace,
		c:         c,
	}
	l, err := NewKubeLock(annotationKey, ownerID, ttl, helper.daemonSetGet, helper.daemonSetUpdate)
	if err != nil {
		return nil, maskAny(err)
	}
	return l, nil
}

type k8sHelper struct {
	name      string
	namespace string
	c         *kc.Client
}

func (h *k8sHelper) daemonSetGet() (annotations map[string]string, resourceVersion string, extra interface{}, err error) {
	var daemonSet v1beta1.DaemonSet
	ctx := context.Background()
	if err := h.c.Get(ctx, h.namespace, h.name, &daemonSet); err != nil {
		return nil, "", nil, maskAny(err)
	}
	md := daemonSet.GetMetadata()
	return md.GetAnnotations(), md.GetResourceVersion(), &daemonSet, nil
}

func (h *k8sHelper) daemonSetUpdate(annotations map[string]string, resourceVersion string, extra interface{}) error {
	daemonSet, ok := extra.(*v1beta1.DaemonSet)
	if !ok {
		return maskAny(fmt.Errorf("extra must be *DaemonSet"))
	}
	md := daemonSet.GetMetadata()
	md.Annotations = annotations
	md.ResourceVersion = kc.String(resourceVersion)
	ctx := context.Background()
	if err := h.c.Update(ctx, daemonSet); err != nil {
		return maskAny(err)
	}
	return nil
}
