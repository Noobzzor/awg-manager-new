# Roadmap: gates без процентов

Продукт: PRODUCT_SPEC.md. Оперативное состояние: PROJECT_STATUS.md. Проверки: TEST_MATRIX.md. Задачи: TASKS.md.

| Gate | Критерий завершения | Состояние |
|---|---|---|
| M0 — воспроизводимая основа | Исходники и dependency pins сохранены; Linux runner; fixtures проверены независимо; актуальный статус/evidence; найденные contract bugs закрыты | IN PROGRESS |
| M1 — single-client DIRECT/BLOCK | AWG client -> ingress -> policy match -> expected outbound -> HTTP/hash response; BLOCK не достигает sink; нет interception bypass или fixture workaround | NOT PROVEN |
| M2 — VPN | Трафик выбранного правила достигает VPN sink/egress; ответ возвращается; выключенный VPN не даёт DIRECT fallback | NOT PROVEN |
| M3 — WARP | Валидный WARP provider, выбранный выход и ответы подтверждены; MTU/DNS и failure fail-closed проверены | NOT PROVEN |
| M4 — multi-client / isolation | Разные source profiles работают одновременно; spoofing, revoked peer и доступ к admin запрещены; политики клиентов не смешиваются | NOT PROVEN |
| M5 — safe apply / persistence / recovery | Store/slot crash consistency, rollback, restart, backup/restore; последняя рабочая политика восстанавливается без секретных утечек | PARTIAL, NOT ACCEPTED |
| M6 — real VPS E2E | Удалённое устройство проверяет четыре действия, DNS, failure, restart/revoke/restore на согласованной VPS среде | BLOCKED BY PRIOR GATES AND DEPLOY AUTHORIZATION |
| M7 — stable functional release | Обязательные пользовательские сценарии и browser smoke приняты, независимый review без открытых критических findings | NOT PROVEN |
| M8 — production hardening | Эксплуатационные лимиты, мониторинг, обновления, эксплуатационная документация | BACKLOG |

Безопасность не откладывается до M8: ключи, fail-closed, client/admin isolation и права доступа обязательны в соответствующих ранних gates. QoS не становится новым обязательным требованием только из-за примерной разбивки milestones.

Порядок gate не означает, что существующие проверенные компоненты нужно переписывать. Старт — с текущего diff и незавершённых задач, не с нуля.
