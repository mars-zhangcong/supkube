// Tiny shared test helpers — kept in one file so each *_test.go stays
// focused on its subject.
package fingerprint

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func metaGet() metav1.GetOptions { return metav1.GetOptions{} }
