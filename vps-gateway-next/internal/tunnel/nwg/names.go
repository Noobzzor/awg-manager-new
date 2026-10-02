package nwg

import (
	"fmt"
	"regexp"
	"strconv"
)

// MaxTunnels — ёмкость индексов WireguardN в NDMS: Wireguard0..Wireguard99.
// Подтверждено автором проекта — NDMS принимает индексы WireguardN до 99.
// Отдельный, более узкий предел существует только для пути через awg_proxy:
// kmod держит 16 одновременных прокси-слотов (AWG_MAX_TUNNELS в
// kmod/awg-proxy/src/proxy.h) и срабатывает при СТАРТЕ туннеля с обфускацией
// на прошивках без нативного ASC, а не при создании интерфейса.
const MaxTunnels = 100

var reNDMSCreated = regexp.MustCompile(`"Wireguard(\d+)" interface created`)

type NWGNames struct {
	Index     int
	NDMSName  string
	IfaceName string
}

func NewNWGNames(index int) NWGNames {
	return NWGNames{
		Index:     index,
		NDMSName:  fmt.Sprintf("Wireguard%d", index),
		IfaceName: fmt.Sprintf("nwg%d", index),
	}
}

func ParseNDMSCreatedName(output string) (index int, ndmsName string, err error) {
	matches := reNDMSCreated.FindStringSubmatch(output)
	if matches == nil {
		return 0, "", fmt.Errorf("nwg: cannot parse NDMS created name from %q", output)
	}
	idx, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", fmt.Errorf("nwg: invalid index in %q: %w", output, err)
	}
	return idx, fmt.Sprintf("Wireguard%d", idx), nil
}
