# Матрица доказательств

PASS / FAIL / NOT TESTED / BLOCKED / STALE / N/A. PASS относится только к зафиксированной версии и acceptance criteria. После изменения соответствующего пути — повторная проверка или STALE. N/A означает неприменимость, не успех.

| Scenario | Config | Unit | Integration | Packet | VPS |
|---|---|---|---|---|---|
| Transport whitespace | NOT TESTED | STALE (до переноса GREEN) | N/A | N/A | N/A |
| Gateway sniff/order | NOT TESTED | STALE (до переноса GREEN) | NOT TESTED | NOT TESTED | NOT TESTED |
| Slot filename migration | NOT TESTED | NOT TESTED | NOT TESTED | N/A | NOT TESTED |
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
