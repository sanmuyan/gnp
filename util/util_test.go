package util

import (
	"fmt"
	"testing"
)

func TestIsAllowPort(t *testing.T) {
	fmt.Println(IsAllowPort("1000-2000,3000", "3000"))
	fmt.Println(IsAllowPort("1000-2000,3000", "2000"))
	fmt.Println(IsAllowPort("1000-2000,3000", "3001"))
	fmt.Println(IsAllowPort("1000-2000,3000", "2001"))
}
