package filler

import (
	"testing"
)

func TestSearchShow(t *testing.T) {
	tests := []struct {
		query string
		want  Show
	}{
		{
			query: "Naruto",
			want: Show{
				Name: "Naruto",
				URL:  "https://www.animefillerlist.com/shows/naruto",
			},
		},
		{
			query: "Attack on Titan",
			want: Show{
				Name: "Attack on Titan",
				URL:  "https://www.animefillerlist.com/shows/attack-titan",
			},
		},
		{
			query: "Naruto Shippuden",
			want: Show{
				Name: "Naruto Shippuden",
				URL:  "https://www.animefillerlist.com/shows/naruto-shippuden",
			},
		},
		{
			query: "Boruto: Naruto Next Generations",
			want: Show{
				Name: "Boruto: Naruto Next Generations",
				URL:  "https://www.animefillerlist.com/shows/boruto-naruto-next-generations",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := SearchShow(tt.query)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got.Name != tt.want.Name {
				t.Fatalf("expected name: %s, got: %s", tt.want.Name, got.Name)
			}

			if len(got.Episodes) == 0 {
				t.Fatalf("expected episodes, got none")
			}

			t.Logf("Found show: %+v", got)
		})
	}
}
