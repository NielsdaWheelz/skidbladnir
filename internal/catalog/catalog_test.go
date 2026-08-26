package catalog

import "testing"

func TestCatalogueProvidesOrderedUniqueRuntimeCharacters(t *testing.T) {
	catalogue, err := decode([]byte(`[
  {"key":"norse.modsognir","displayName":"Móðsognir","tradition":"OldNorse","source":{"work":"Vǫluspá","locus":"st. 10"}},
  {"key":"norse.durinn","displayName":"Durinn","tradition":"OldNorse","source":{"work":"Vǫluspá","locus":"st. 10"}}
]`))
	if err != nil {
		t.Fatalf("decode valid catalogue: %v", err)
	}
	characters := catalogue.Characters()
	if len(characters) != 2 || characters[0].Key != "norse.modsognir" || characters[1].NameSuffix != "durinn" {
		t.Fatalf("catalogue order or runtime identity changed: %+v", characters)
	}

	if _, err := decode([]byte(`[
  {"key":"norse.durinn","displayName":"Durinn"},
  {"key":"tolkien.durinn","displayName":"Another Durinn"}
]`)); err == nil {
		t.Fatal("duplicate generated-session suffix was accepted")
	}
	if _, err := decode([]byte("[{\"key\":\"norse.durinn\",\"displayName\":\"\xff\"}]")); err == nil {
		t.Fatal("invalid UTF-8 catalogue content was accepted")
	}
}
