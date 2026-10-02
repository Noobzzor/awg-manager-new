# Матрица доказательств

PASS / FAIL / NOT TESTED / BLOCKED / STALE / N/A. PASS относится только к зафиксированной версии и acceptance criteria. После изменения соответствующего пути — повторная проверка или STALE. N/A означает неприменимость, не успех.

| Scenario | Config | Unit | Integration | Packet | VPS |
|---|---|---|---|---|---|
| Transport whitespace | NOT TESTED (live engine) | PASS (new Linux copy) | N/A | N/A | N/A |
| Gateway sniff/order | NOT TESTED (live engine) | PASS (new Linux copy, merged order) | NOT TESTED | NOT TESTED | NOT TESTED |
| Slot filename migration | N/A | PASS (RED/GREEN, ten layouts) | PASS (validator/runtime merge, review pending) | N/A | NOT TESTED |
| Migration error startup gate | N/A | PASS (RED/GREEN, portable/runtime initialization) | PASS (related composition tests) | N/A | NOT TESTED |
| Migration state-directory links | N/A | PASS (RED/GREEN, pending/disabled; external bytes preserved) | N/A | N/A | N/A |
| Pre-operator production directory preflight | N/A | PASS (RED/GREEN, local/runtime root/disabled/pending) | PASS (migratable external JSON and file set preserved) | N/A | NOT TESTED |
| Uninstall initialization failure / error status | N/A | PASS (mock-only stages/order/one-shot/preflight) | NOT TESTED (real uninstall) | N/A | NOT TESTED |
| Migration partial retry / surviving engine | N/A | NOT TESTED | NOT TESTED | N/A | NOT TESTED |
| Fixture inside/outside alias control | N/A | N/A | PASS (историческая fixture only) | NOT TESTED (новая topology) | N/A |
| DIRECT single client | NOT TESTED | NOT TESTED | NOT TESTED | BLOCKED (fixture confound) | NOT TESTED |
| BLOCK single client | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| VPN | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| WARP | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Multi-client / isolation / revoke | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Safe apply / persistence / restart | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED | NOT TESTED |
| Backup/restore | NOT TESTED | NOT TESTED | NOT TESTED | N/A | NOT TESTED |
| Browser UI | N/A | NOT TESTED | NOT TESTED | N/A | NOT TESTED |

NOT TESTED здесь означает отсутствие принятого актуального evidence в новой рабочей копии, не отсутствие реализации и не утверждение, что тест раньше никогда не проходил. Исторические results остаются локально в прежнем журнале/review-packs и не публикуются массово.

## Evidence index

Новые результаты записывать со source hash/commit, runner/image ID, командой, exit code и точным scope. Sensitive/raw logs держать локально; в public repository публиковать только проверенный redacted summary.

Текущий локальный пакет 52-next-development-bootstrap: migration RED exit 1; migration GREEN exit 0; orchestrator/configmerge/policy/forwarding/ingress package tests exit 0. SHA256 baseline copy manifest сохранён локально. Публичный baseline пока содержит только documentation/license files, не эти исходники и не raw logs.

Пакет 53-migration-review-fixes: symlink RED/GREEN; production-bootstrap RED/GREEN; cmd/awg-manager плюс пять связанных packages PASS; go vet PASS; final focused tests и gofmt clean. SOURCE_AND_RUNNER.json фиксирует hashes изменённых файлов и pinned image. Истёкший disposable runner был поднят снова по exact owned ID; protected containers readback — running/healthy. Не считать harness failure «container not running» production test failure или GREEN.

Пакет 55-production-preflight-cleanup (preflight-*): actual production symlink regression RED/GREEN до/после переноса проверки перед NewOperator; singbox/orchestrator suites и vet PASS, formatting clean. Cleanup-* результаты и consolidated cmd suite требуют отдельного принятия после worker completion; этот preflight slice не закрывает cleanup или packet gate.

55 cleanup/consolidation принято координатором: actual cmd/awg-manager + related suites повторены, 13 test-bearing packages PASS; internal/cleanup has no standalone tests. Vet всех 14 targets PASS. Cleanup tests не выполняют реальное удаление ресурсов. Migration extended cases, real engine convergence и packet/VPS proof не закрыты этим результатом.
