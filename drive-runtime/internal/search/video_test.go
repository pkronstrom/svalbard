package search

import "testing"

func TestVideoURLSeeksArchivedMedia(t *testing.T) {
	url, ok := VideoURL("http://localhost:9000", "videos.zim/folder/first video.webm", "time/30160")
	if want := "http://localhost:9000/content/videos/folder/first%20video.webm#t=30.160"; !ok || url != want {
		t.Fatalf("VideoURL = %q, %t, want %q", url, ok, want)
	}
	if _, ok := VideoURL("http://localhost:9000", "videos.zim/folder/video.webm", "time/-1"); ok {
		t.Fatal("negative video timestamp accepted")
	}
}
