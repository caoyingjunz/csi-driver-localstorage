package Annotationimageset

const (
	Annotation			string = "img.jixingxing.io/imageset"
)

var (
	// Init SafeSet which contains the HPA Average Utilization / Value
	kset = NewSafeSet("img.jixingxing.io/imageset")
)

// To ensure whether we need to maintain the HPA
func IsNeedForIMGs(annotations map[string]string) bool {
	if annotations == nil || len(annotations) == 0 {
		return false
	}

	for aKey := range annotations {
		if kset.Has(aKey) {
			return true
		}
	}
	return false
}

func CheckAnnotation(annotations map[string]string) error {
	var err error
	_, exists := annotations[Annotation]
	if exists {
		return nil
	}
	return err
}