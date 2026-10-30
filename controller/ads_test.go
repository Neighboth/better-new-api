package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func withOptionMap(t *testing.T, values map[string]string) {
	t.Helper()
	previous := common.OptionMap
	common.OptionMap = values
	t.Cleanup(func() { common.OptionMap = previous })
}

func TestLegacyAdStaysOnTheBlog(t *testing.T) {
	// An ad written before the console existed carries no surface. The upgrade
	// must not move it: it belongs to the blog, the surface it was written for.
	ad := customAd{ID: "old", Image: "image", URL: "https://example.com"}

	require.Equal(t, []string{AdSurfaceBlog}, placementsOf(ad))
	require.True(t, adOnSurface(ad, AdSurfaceBlog))
	require.False(t, adOnSurface(ad, AdSurfaceConsole))
}

func TestAdSurfacesAreNormalised(t *testing.T) {
	ad := customAd{
		ID:         "campaign",
		Placements: []string{"console", "blog", "console", "unknown"},
	}
	require.Equal(t, []string{AdSurfaceBlog, AdSurfaceConsole}, placementsOf(ad))

	onlyConsole := customAd{ID: "console", Placements: []string{"console"}}
	require.False(t, adOnSurface(onlyConsole, AdSurfaceBlog))
	require.True(t, adOnSurface(onlyConsole, AdSurfaceConsole))

	unknown := customAd{ID: "unknown", Placements: []string{"nonsense"}}
	require.Equal(t, []string{AdSurfaceBlog}, placementsOf(unknown))
}

func TestInstallationSurfaces(t *testing.T) {
	withOptionMap(t, map[string]string{})
	// An installation that never saved the option keeps the blog only.
	require.Equal(t, []string{AdSurfaceBlog}, adSurfaces())
	require.True(t, surfaceEnabled(AdSurfaceBlog))
	require.False(t, surfaceEnabled(AdSurfaceConsole))

	withOptionMap(t, map[string]string{"AdsSurfaces": "blog, console,blog,wat"})
	require.Equal(t, []string{AdSurfaceBlog, AdSurfaceConsole}, adSurfaces())
	require.True(t, surfaceEnabled(AdSurfaceConsole))

	withOptionMap(t, map[string]string{"AdsSurfaces": "console"})
	require.False(t, surfaceEnabled(AdSurfaceBlog))
	require.True(t, surfaceEnabled(AdSurfaceConsole))

	// An option that names no surface keeps the blog and never turns the
	// console on by accident.
	withOptionMap(t, map[string]string{"AdsSurfaces": "nonsense"})
	require.True(t, surfaceEnabled(AdSurfaceBlog))
	require.False(t, surfaceEnabled(AdSurfaceConsole))
}

func TestAdsAreFilteredBySurface(t *testing.T) {
	withOptionMap(t, map[string]string{
		"AdsSurfaces": "blog,console",
		"CustomAds": `[
			{"id":"legacy","image":"image","url":"https://example.com"},
			{"id":"both","image":"image","url":"https://example.com","placements":["blog","console"]},
			{"id":"console","image":"image","url":"https://example.com","placements":["console"]},
			{"id":"blog","image":"image","url":"https://example.com","placements":["blog"]},
			{"id":"broken","image":"","url":"https://example.com","placements":["console"]}
		]`,
	})

	visible := func(surface string) []string {
		ids := make([]string, 0)
		for _, ad := range getCustomAds() {
			if ad.Image == "" || ad.URL == "" || !adOnSurface(ad, surface) {
				continue
			}
			ids = append(ids, ad.ID)
		}
		return ids
	}

	// The blog keeps every ad that names the blog and the legacy ad.
	require.Equal(t, []string{"legacy", "both", "blog"}, visible(AdSurfaceBlog))
	// The console receives the ads that name it, and never the broken one.
	require.Equal(t, []string{"both", "console"}, visible(AdSurfaceConsole))
}
