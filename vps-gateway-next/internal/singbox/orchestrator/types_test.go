package orchestrator

import "testing"

func TestKnownSlotsIncludesLocalProxyBeforeTunnelSlots(t *testing.T) {
	slots := KnownSlots()
	if len(slots) < 2 || slots[0].Slot != SlotBase || slots[1].Slot != SlotLocalProxy {
		t.Fatalf("first slots = %+v, want base then local proxy", slots[:2])
	}
	if slots[1].Filename != "05-local-proxy.json" || !slots[1].AlwaysOn {
		t.Fatalf("local proxy metadata = %+v", slots[1])
	}
}

func TestKnownSlotsIncludesDNSRewritesBeforeRouter(t *testing.T) {
	slots := KnownSlots()
	idxRewrites, idxRouter := -1, -1
	for i, m := range slots {
		switch m.Slot {
		case SlotDNSRewrites:
			idxRewrites = i
		case SlotRouter:
			idxRouter = i
		}
	}
	if idxRewrites < 0 {
		t.Fatal("SlotDNSRewrites not registered")
	}
	if idxRewrites >= idxRouter {
		t.Errorf("slot order: rewrites=%d router=%d", idxRewrites, idxRouter)
	}
}

func TestKnownSlotsIncludesDNSRoutesAtOwnedPriority(t *testing.T) {
	slots := KnownSlots()
	idxQoS, idxDNSRoutes, idxRouter := -1, -1, -1
	filename := ""
	for i, m := range slots {
		switch m.Slot {
		case SlotQoSRoutes:
			idxQoS = i
		case SlotDNSRoutes:
			idxDNSRoutes = i
			filename = m.Filename
		case SlotRouter:
			idxRouter = i
		}
	}
	if idxDNSRoutes < 0 {
		t.Fatal("SlotDNSRoutes not registered")
	}
	if filename != "19-dns-routes.json" {
		t.Fatalf("SlotDNSRoutes filename = %q", filename)
	}
	if !(idxQoS < idxDNSRoutes && idxDNSRoutes < idxRouter) {
		t.Fatalf("slot order: qos=%d dns-routes=%d router=%d", idxQoS, idxDNSRoutes, idxRouter)
	}
}

func TestKnownSlotsIncludesGatewayPolicyBeforeRouter(t *testing.T) {
	slots := KnownSlots()
	idxGateway, idxRouter := -1, -1
	filename := ""
	for i, m := range slots {
		switch m.Slot {
		case SlotGatewayPolicy:
			idxGateway = i
			filename = m.Filename
		case SlotRouter:
			idxRouter = i
		}
	}
	if idxGateway < 0 {
		t.Fatal("SlotGatewayPolicy not registered")
	}
	if filename != "18-z-gateway-policy.json" || idxGateway >= idxRouter {
		t.Fatalf("gateway slot = (%d, %q), router=%d", idxGateway, filename, idxRouter)
	}
}

func TestKnownSlotsGatewayPolicyPrecedesDNSRoutesInMergedFilenameOrder(t *testing.T) {
	slots := KnownSlots()
	idxQoS, idxGateway, idxDNSRoutes, idxRouter := -1, -1, -1, -1
	filenames := make(map[Slot]string)
	for i, m := range slots {
		filenames[m.Slot] = m.Filename
		switch m.Slot {
		case SlotQoSRoutes:
			idxQoS = i
		case SlotGatewayPolicy:
			idxGateway = i
		case SlotDNSRoutes:
			idxDNSRoutes = i
		case SlotRouter:
			idxRouter = i
		}
	}
	if !(idxQoS >= 0 && idxQoS < idxGateway && idxGateway < idxDNSRoutes && idxDNSRoutes < idxRouter) {
		t.Fatalf("slot order: qos=%d gateway=%d dns-routes=%d router=%d", idxQoS, idxGateway, idxDNSRoutes, idxRouter)
	}
	if !(filenames[SlotQoSRoutes] < filenames[SlotGatewayPolicy] && filenames[SlotGatewayPolicy] < filenames[SlotDNSRoutes]) {
		t.Fatalf("merged filename order: qos=%q gateway=%q dns-routes=%q", filenames[SlotQoSRoutes], filenames[SlotGatewayPolicy], filenames[SlotDNSRoutes])
	}
}
