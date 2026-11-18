package recordinglink

import "testing"

func TestURLAndValid(t *testing.T) {
	const secret = "sek"
	const id = "9822217c-e2ff-4aa4-b16b-005db28a876c"
	got := URL("https://cdp-dmp.ru/", secret, id)
	wantPrefix := "https://cdp-dmp.ru/rest/recordings/" + id + ".mp3?sig="
	if len(got) <= len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("url %q", got)
	}
	sig := got[len(wantPrefix):]
	if !Valid(secret, id, sig) {
		t.Fatal("expected valid")
	}
	if Valid(secret, id, sig+"00") || Valid("other", id, sig) || Valid(secret, "", sig) {
		t.Fatal("expected invalid")
	}
	if URL("", secret, id) != "" || URL("https://cdp-dmp.ru", "", id) != "" {
		t.Fatal("empty parts")
	}
}
