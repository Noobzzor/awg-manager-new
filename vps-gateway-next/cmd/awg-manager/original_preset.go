package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

const (
	originalGeositeBase = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/"
	originalPresetLimit = 8 * 1024 * 1024
)

// Keep the approved catalog identities and payloads independent of the incoming preset.
var originalGeositePins = map[string]string{
	"adobe":          "c1d89441308c4848a1e17a8ce1d5e3ee894d75f2c2e53b483de8eb29687b07c2",
	"akamai":         "a9fb9c153e882ed8c06df0a4f25e833d972996a862e6443a9451963587a8d3b6",
	"apple":          "37f806579cdbd07e746266e0778f1edaf14797e0244e889d6385b6d53cbde495",
	"aws":            "15d07d05945df3c95f2cf41f8f9e8ce0706c1c712f821f319faeb43f496f4837",
	"bbc":            "b7a797ab79f44fa4fbabb1fbb605ded80adce00b34de438ba9698ef983d6d6df",
	"binance":        "b824818daed07f4640c474b39a849a9b3dae9ec6dbb290fdd0d61046efa0f899",
	"blizzard":       "a2be497db89f70be1bce01a945ef68817787c3715076488cf03b79e591317ff4",
	"bluesky":        "18ae2f07aceea03840613b419de77e6d66f1478d1007612527b616445c4cdf19",
	"canva":          "668e0828660b414c8d6c5e0ec6a93162bb4dde651e8125701abbe615e7dfd3d4",
	"category-games": "97df567780ae1f3987ad112c6cc54dfa3650d513a0c0e7baf19da202ceacd3d0",
	"category-media": "e0bb668c771a106657f4c1085e47273616d4ab7bb9bddd5f702cd2a4f2e45521",
	"cloudflare":     "5fa87d2b2ded9b7cd42ba40ae720d739616ff1ebe65804337ac47fb097f9f047",
	"deezer":         "d4e8ee54e3b494b411f7d4d89356128214b71a93b471d3d1874a07c55691f546",
	"disney":         "4320bb412187c690662d23ae20fad699d495acd9f9a5c205a04a3528ecdc6f71",
	"docker":         "2ca3deec04c140ff249e89d46de56e164d5dd01559035f9386bf0af84e8bd14a",
	"duckduckgo":     "e607671a6f06aa1035f19febd8a32286e378ffbe21526f1dde8652c087205917",
	"ea":             "301038c59c16ad810f4cdf95172831333ebb9a4bce9679a9c6eab9fc38851ddb",
	"epicgames":      "60e4ea1b1907cf9034a8e673dd81c8d834812cf8dc368c043fc57b0b1c477e10",
	"facebook":       "25f39c0938e0fa5b44880d7d29c4497557a7e70b13e6fdabd667b0059677045e",
	"figma":          "bca8920bcae8cf69ed822776c8c49c7a4f9685cb978980b676ed5cb05e0ce7d6",
	"github":         "438f12d297174b168b3b6acd4572c360c083c24641736bdaefb4ed2cb3d0295f",
	"gitlab":         "769b9def6b6750258eb972501bf8724d1238cfc39a7f7ec392aa1fe65bf08e9f",
	"google":         "992d7ba91ce9c6e1c248d9ef0d58b2a17281415526b6a560dce6368e4f72e1f2",
	"google-play":    "518c00da23d12431c536463604816c615faec02602aae58165f8a6993dd9bcac",
	"hbo":            "116101423791d014f6c2640d146043723ed60daec94874f1e4f26273116bc86c",
	"hulu":           "fca3136453122e2a219cf55d0f08777de700d4329c4c0130b0e74f294f32da77",
	"jetbrains":      "80d133945e182408e69d5a23bea78788bc74748a57ed6d7c4417ded568b0e045",
	"kinopub":        "1ab7c61973b5fb4c579633c51205e5423ae28d064c47fc7fe88c15d916126710",
	"medium":         "8e8772746b392b9f4e3aef33889ee27a1bd6aabe33f7f9731cf537c3bfd4c571",
	"meta":           "de1d4616602378940ce848b8a8ea7172dde2dcb4a1aef3b9710d0f5a5990f280",
	"microsoft":      "7827c1d3394074fbfa60ee19b2471a8f88471bf5ff80ac8c420847de5a1174a3",
	"notion":         "f802608674b7535b09ab0b693ac3e023a6418150834ecce074f2eb1d8ae1a357",
	"nvidia":         "c1b6bc8e62610421665682398e42bb65fa0f7720edeb2e229763b7a2694ade3d",
	"oculus":         "bf18733a0ecaffa3545a5f33c11fd1c25da7a23b673f43f49f298fb5c96a1398",
	"patreon":        "0da6b4c92d54cd950b5096a633b505cd3a16c1798b567e4d5730052b9d9d339e",
	"paypal":         "9bf0331b219c6209a226edc95d44862f2ad96c2ebe21e2a4a45d110422e371e1",
	"perplexity":     "345e8d4175571b6d1c6e973c76e725d9b30317a7ef8b39dc1f032f8f2cd12d58",
	"pinterest":      "b312d6200bf57417d393051a17c54873f628236689b6a5ad2152ae68eda20741",
	"playstation":    "b1ec5130587eb5d2dfd53720ee36c05838f31e2f4ea6070984c0af563bdfb678",
	"primevideo":     "5e9adb65f4eeb6b69d7bcf12e6c0c41c8b2b6c4196e5078186024c99f0ef5299",
	"reddit":         "11611c5c643ea855a7624ca2f1ab39d710dffbf42ca1c2b4d8570a8f07de83a9",
	"signal":         "71ba0a7df49a68b24e90ef0fdcf394aa6f6e22526679d5d58fbff7aec15de6b1",
	"slack":          "520bea57deaa0b918bdef448475f888987cf140c3aeeb55a3e90534c5f3b1cea",
	"soundcloud":     "64c00d7e2157fc24955e9544912a9e33055b982a97eb09b1d491894b687bd0c6",
	"spotify":        "784a099fc5c64cbacca85c46017bc58c0ad34b9919fcf4b831294f66ef8fe328",
	"steam":          "1038095f25bdf99c98d70ce4410b4ad11c696bbb922c3991d76cd906ba8d5f2f",
	"threads":        "17c0bbdf82ba59cbde1254297c836b7cb731eb3508211dbf55fa4040630e3377",
	"tidal":          "0d5f77e5a5ad99266222af5f4e296547c162069fb6f349eaadbd68fac729242d",
	"twitch":         "85e3b8d528cff47c0ac486f754ebc453fb5f1b78b8f9f1c7a22e398d89065bbf",
	"vercel":         "6f98ef8cf83b3a4991556f050cc40751b9f6e46e6dd83f68c2c020a07508fb57",
	"vimeo":          "4b692399c90bc7696702e176a37e4692b2d09ed7679770ba58df4b9e73caaf53",
	"wikimedia":      "0db48f644bb7088f690bb581b7ca9d02a924facf92506c587aa13a7da4b34e1c",
	"xbox":           "5d81ec165a4d2a5283578cbe293490a37eca24bf0d979b49d99f48cd27b8fd3f",
	"xiaomi":         "2f7f505f521e00ed8c17f98fd0deead233713128d17b114839236c5c80a98c01",
	"zoom":           "49175e0922578e9d2667a91565ec89f3c19c2318ccab5c1798cce5806f718d9e",
}

// The catalog keeps these historical IDs or tags even when the file name differs.
var originalGeositeExceptions = map[string]struct {
	tag, file, digest string
}{
	"category-ai": {"geosite-category-ai-!cn", "geosite-category-ai-!cn.srs", "0050a0d90e239884b9c9161d5da8edfff56c6adfb17b6a7ffdd57897928d42cc"},
	"ads":         {"geosite-category-ads-all", "geosite-category-ads-all.srs", "5577fd713d8479b6df9bcfe5832eab9d6d4fd5a34ea5748460618ed30526cb0c"},
	"porn":        {"geosite-category-porn", "geosite-category-porn.srs", "21d2d79413ab1963f7cc45d484b069504e9c7fa0d44a2f25c6c76cdfa333babf"},
	"npm":         {"geosite-npm", "geosite-npmjs.srs", "7ad650fd9b469bf66d0bfbbbb66e39b5d4cf3a14c3f7f2ea0ee70faab51725cb"},
}

const originalHoaxisCatalogBase = "https://repo.hoaxisr.ru/rulesets/srs/"
const originalHoaxisSourceBase = "https://raw.githubusercontent.com/hoaxisr/rulesets/2879d42b8a701cae7f1caf886e403e02ba40ed22/srs/"

var originalHoaxisPins = map[string]struct {
	tag, file, digest string
}{
	"anthropic":             {"geosite-anthropic", "claude.srs", "5a0cd5d4bf0d3d71a55a43362787e52d81e2fefe71da0732b3726a0553fe241c"},
	"copilot":               {"geosite-copilot", "copilot.srs", "9f017f25b209b4a42d94b25389b206c8a0273d8c9b66a4eec3e055db77e62457"},
	"discord":               {"geosite-discord", "discord-full.srs", "d6f663aea9d3da4e5c3d0e339dcd73c987873483b3f1b435b3b6d4e2d57fe8bf"},
	"gemini":                {"geosite-google-gemini", "gemini.srs", "48292ec3fa81752a682205950e5552967ae51fc08db25f121e691d3c396e0a83"},
	"grok":                  {"geosite-grok", "grok.srs", "74df5e7ceed987fede967e6453149575d2973510e6d21e30482a5c773714e492"},
	"instagram":             {"geosite-instagram", "instagram.srs", "815d98913e07c2c494ffff7e4b6585421da3fb230897163d6076aaa241a220b4"},
	"linkedin":              {"geosite-linkedin", "linkedin.srs", "84cf81e34647e27a8f4658d00b41a477aca66dec9c50986b57d4ab4c60187078"},
	"netflix":               {"geosite-netflix", "netflix.srs", "d4b82427e97f7fb99760824f99bdbd2dbe8c80d30c6115b5aedc8d015df835ee"},
	"nintendo":              {"geosite-nintendo", "nintendo.srs", "9017841e7162547b35129df2783a205fa5cec6cc7bf903a0859a48cee5c1aca8"},
	"openai":                {"geosite-openai", "openai.srs", "44aab89f804163efbb8f8b0ddcf4dd6720ec631e19248943e957841881e8e323"},
	"rkn":                   {"geosite-rkn", "rkn.srs", "e8d388e34e6e844f0d21768d2fe00965ef3b10d339b11fc5cd9ebba22ce0a8d5"},
	"roblox":                {"geosite-roblox", "roblox.srs", "efdd7b05cfab39a8a7633ea615b5279726dd82698f5c9a41b2c802914e5b5a4b"},
	"telegram":              {"geosite-telegram", "telegram.srs", "d525b0e4196d9121cf0ad387784fc390520e61b02ab36e4b4d6d1920c7c24833"},
	"tiktok":                {"geosite-tiktok", "tiktok.srs", "0d6eab857101dae85aa3ed6a3599835a6e7c7e32118119dc52ff12b92f98f0df"},
	"unavailable-in-russia": {"geosite-unavailable-in-russia", "unavailable-in-russia.srs", "f7cb20e68f240a479263db8619a4190195e77b18ab577f1fa44d03034c038140"},
	"whatsapp":              {"geosite-whatsapp", "whatsapp.srs", "e3d4efc5d5e8bc453287351c0c6efc27908ba51279dbd11ed6d37cceffcd7e2b"},
	"x":                     {"geosite-x", "x.srs", "2ad3b7eef8ef9247bcf756fc07a0c65618bd1aa7ba35557913aa978e5408ed56"},
	"youtube":               {"geosite-youtube", "youtube.srs", "f4f87eed01e6aba1ea29ec209787d708644f15fdc4f92543342ae9ccebb932f0"},
}

// resolveOriginalPresetRuleSets runs under the original editor's writer lock.
func (a *app) resolveOriginalPresetRuleSets(ctx context.Context, preset router.Preset) ([]router.RuleSet, error) {
	digest := originalGeositePins[preset.ID]
	tag := "geosite-" + preset.ID
	file := tag + ".srs"
	if pin, ok := originalGeositeExceptions[preset.ID]; ok {
		tag, file, digest = pin.tag, pin.file, pin.digest
	}
	catalogBase, downloadBase := originalGeositeBase, originalGeositeBase
	if pin, ok := originalHoaxisPins[preset.ID]; ok {
		tag, file, digest = pin.tag, pin.file, pin.digest
		catalogBase, downloadBase = originalHoaxisCatalogBase, originalHoaxisSourceBase
	}
	ref := router.RuleRef{Tag: tag, URL: catalogBase + file}
	if digest == "" || len(preset.RuleSets) != 1 || preset.RuleSets[0] != ref {
		return nil, fmt.Errorf("this preset has no verified local rule-set")
	}
	if err := a.verifyOriginalOwner(ctx); err != nil {
		return nil, err
	}
	dir := filepath.Join(a.singboxConfigDir, "rule-sets", "presets")
	path := filepath.Join(dir, "sha256-"+digest+".srs")
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0400 || info.Size() < 1 || info.Size() > originalPresetLimit {
			return nil, fmt.Errorf("cached preset file has unsafe attributes")
		}
		raw, err := os.ReadFile(path)
		if err != nil || !matchesOriginalPresetDigest(raw, digest) {
			return nil, fmt.Errorf("cached %s preset differs from its verified file", preset.ID)
		}
	case os.IsNotExist(err):
		ref.URL = downloadBase + file
		if err := a.downloadOriginalPreset(ctx, ref, digest, path); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return []router.RuleSet{{Tag: tag, Type: "local", Format: "binary", Path: path}}, nil
}

func matchesOriginalPresetDigest(raw []byte, digest string) bool {
	return fmt.Sprintf("%x", sha256.Sum256(raw)) == digest
}

func (a *app) downloadOriginalPreset(ctx context.Context, ref router.RuleRef, digest, path string) error {
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref.URL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s preset: %w", ref.Tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s preset: HTTP %d", ref.Tag, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, originalPresetLimit+1))
	if err != nil || len(raw) > originalPresetLimit || !matchesOriginalPresetDigest(raw, digest) {
		return fmt.Errorf("downloaded %s preset differs from its verified file", ref.Tag)
	}
	if err := a.verifyOriginalOwner(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := storage.AtomicWritePerm(path, raw, 0400); err != nil {
		return err
	}
	for _, parent := range []string{dir, filepath.Dir(dir), a.singboxConfigDir} {
		handle, err := os.Open(parent)
		if err != nil {
			return err
		}
		syncErr := handle.Sync()
		closeErr := handle.Close()
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
