# Существенные решения

## D-001 — серверный продукт сохраняется
- Decision: AWG ingress + серверный policy engine + web management. Не переключаться на локальный клиент/конструктор профилей.
- Reason: пользователь подтвердил, что решения и управление должны быть на VPS.
- Evidence: утверждённая PRODUCT_SPEC.md и исходный VPS Gateway план.
- Reconsider: только по решению Product Owner.

## D-002 — не лечить fixture ограничение production workaround
- Decision: не добавлять production SNAT/bind по outside-subnet Docker --internal A/B.
- Reason: проблема воспроизведена обычным sender/sink без продукта.
- Evidence: локальный 50-internal-bridge-control; bridge/inside alias success, outside alias timeout, sink-local listener success. Это не exact host DROP counter evidence.
- Reconsider: только при новом независимом доказательстве production egress defect.

## D-003 — TUN provisional, не архитектурная догма
- Decision: сохранить текущий TUN путь на время исправления fixture и regressions; не переключать одновременно interception, topology, NAT и policy.
- Reason: настоящий product failure в исходной fixture не изолирован; переписывание сейчас увеличит число переменных.
- Evidence: runtime policy/SYN historical evidence не равно reply proof; общий functional gate открыт.
- Reconsider: минимальная валидная topology показывает воспроизводимый TUN дефект или неприемлемую сложность; нужен независимый review.

## D-004 — публичная новая папка, старый код сохранён
- Decision: publish только vps-gateway-next в Noobzzor/awg-manager-new; source/license/pins сохраняются, конфиги/секреты/raw logs не копируются.
- Reason: пользователь выбрал отдельный repository и новую папку для дальнейшей разработки.
- Evidence: exact remote commit/tree readback обязателен; до него публикация не считается выполненной.
- Reconsider: меняется видимость или место хранения по решению пользователя.
