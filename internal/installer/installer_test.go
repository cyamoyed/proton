package installer

import "testing"

func TestPathContains(t *testing.T) {
	if !pathContains(`C:\\Tools;C:\\Users\\me\\Proton\\bin`, `C:\\Users\\me\\Proton\\bin`, ";") {
		t.Fatal("expected exact PATH entry to be found")
	}
	if pathContains(`/usr/bin:/home/me/bin-extra`, `/home/me/bin`, ":") {
		t.Fatal("partial PATH entry must not match")
	}
}
