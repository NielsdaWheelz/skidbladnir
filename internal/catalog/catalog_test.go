package catalog

import "testing"

func TestCatalogueProvidesOrderedCharactersIndependentOfTmuxNames(t *testing.T) {
	catalogue, err := decode([]byte(`[
  {"key":"norse.modsognir","displayName":"Móðsognir","tradition":"OldNorse","source":{"work":"Vǫluspá","locus":"st. 10"}},
  {"key":"norse.durinn","displayName":"Durinn","tradition":"OldNorse","source":{"work":"Vǫluspá","locus":"st. 10"}},
  {"key":"tolkien.durinn","displayName":"Another Durinn","tradition":"ModernLiterature","source":{"work":"The Hobbit","locus":"ch. 1"}}
]`))
	if err != nil {
		t.Fatalf("decode valid catalogue: %v", err)
	}
	characters := catalogue.Characters()
	if len(characters) != 3 || characters[0] != (Character{Key: "norse.modsognir", DisplayName: "Móðsognir"}) || characters[2] != (Character{Key: "tolkien.durinn", DisplayName: "Another Durinn"}) {
		t.Fatalf("catalogue order or character values changed: %+v", characters)
	}
	if _, err := decode([]byte("[{\"key\":\"norse.durinn\",\"displayName\":\"\xff\"}]")); err == nil {
		t.Fatal("invalid UTF-8 catalogue content was accepted")
	}
}
