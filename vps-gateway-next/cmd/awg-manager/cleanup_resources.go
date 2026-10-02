package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/hoaxisr/awg-manager/internal/cleanup"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// cleanupAction adapts tracked callbacks without changing service ordering.
type cleanupAction func(context.Context) error

func (f cleanupAction) Cleanup(ctx context.Context) error        { return f(ctx) }
func (f cleanupAction) CleanupAll(ctx context.Context) error     { return f(ctx) }
func (f cleanupAction) DeleteIfExists(ctx context.Context) error { return f(ctx) }
func (f cleanupAction) Save(ctx context.Context) error           { return f(ctx) }

type cleanupDelete func(context.Context, string) error

func (f cleanupDelete) Delete(ctx context.Context, id string) error { return f(ctx, id) }

type cleanupList func() ([]storage.AWGTunnel, error)

func (f cleanupList) List() ([]storage.AWGTunnel, error) { return f() }

// CleanupAll historically logs and swallows dependency errors. Track them at
// this command boundary while leaving the service's best-effort contract intact.
func cleanupResources(ctx context.Context, deleter cleanup.TunnelDeleter, lister cleanup.TunnelLister, dns cleanup.DnsRouteCleaner, managed cleanup.ManagedServerCleaner, policies cleanup.PolicyCleaner, clients cleanup.ClientRouteCleaner, cleaner cleanup.SingboxCleaner, saver cleanup.ConfigSaver) error {
	var failures []error
	record := func(stage string, err error) error {
		if err != nil {
			failures = append(failures, fmt.Errorf("cleanup %s: %w", stage, err))
		}
		return err
	}
	track := func(stage string, f func(context.Context) error) cleanupAction {
		return func(ctx context.Context) error { return record(stage, f(ctx)) }
	}
	if deleter != nil {
		original := deleter
		deleter = cleanupDelete(func(ctx context.Context, id string) error { return record("tunnel "+id, original.Delete(ctx, id)) })
	}
	if lister != nil {
		original := lister
		lister = cleanupList(func() ([]storage.AWGTunnel, error) {
			items, err := original.List()
			return items, record("list tunnels", err)
		})
	}
	if dns != nil {
		dns = track("DNS routes", dns.CleanupAll)
	}
	if managed != nil {
		managed = track("managed server", managed.DeleteIfExists)
	}
	if policies != nil {
		policies = track("access policies", policies.CleanupAll)
	}
	if clients != nil {
		clients = track("client routes", clients.CleanupAll)
	}
	if cleaner != nil {
		cleaner = track("sing-box", cleaner.Cleanup)
	}
	if saver != nil {
		saver = track("save configuration", saver.Save)
	}
	record("resources", cleanup.New(deleter, lister, dns, managed, policies, clients, cleaner, saver).CleanupAll(ctx))
	return errors.Join(failures...)
}
