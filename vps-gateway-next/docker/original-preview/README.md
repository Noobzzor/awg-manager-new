# AWG Manager на Debian

Комплект собирает панель из исходников текущей ветки. Для запуска используются
sing-box и AmneziaWG из базового образа из архива AWG от 23 октября. Попроси
автора проекта прислать этот архив вместе со ссылкой на исходники.

Нужны Debian x86_64, Docker Engine с Compose, устройство /dev/net/tun и свободный
входящий UDP-порт 51820. Для подключения из интернета у сервера должен быть
доступный IPv4 или домен. На время сборки нужен интернет для загрузки зависимостей.

## Сборка и запуск

1. Распакуй архив AWG от 23 октября. В папке с его файлами выполни:

   ```sh
   docker load -i install/image.tar.gz
   docker image inspect --format '{{.Id}}' awgm-personal-trial:20261021-dns1
   ```

   Ожидаемый ID — `sha256:2178524bc069afd4aa2c5b639e6acb73154f96a24247db07c33d685fa0b89929`.

2. В скачанных исходниках открой папку `vps-gateway-next/docker/original-preview`.
   Выполни команды и подставь адрес своего сервера вместо `192.0.2.10`:

   ```sh
   export AWG_PREVIEW_DIR="$HOME/.local/share/awgm-original-preview"
   export AWG_PANEL_UID="$(id -u)"
   export AWG_PANEL_GID="$(id -g)"
   export AWG_GATEWAY_PUBLIC_ENDPOINT="192.0.2.10:51820"
   install -d -m 700 "$AWG_PREVIEW_DIR/panel" "$AWG_PREVIEW_DIR/bootstrap"
   docker compose build awg
   docker compose up -d
   docker compose ps
   ```

   При выборе другого внешнего порта задай `AWG_UDP_PORT` и укажи тот же
   порт в `AWG_GATEWAY_PUBLIC_ENDPOINT`.

3. С компьютера, на котором будешь открывать панель, запусти SSH-туннель.
   Подставь имя SSH-пользователя, адрес и **полный путь на сервере**
   к папке, которую создал на шаге 2. Используй того же пользователя,
   чей ID указан в `AWG_PANEL_UID`.

   ```sh
   ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:2222:/home/USER/.local/share/awgm-original-preview/panel/awg.sock USER@SERVER
   ```

   Открой [VPS Gateway](http://127.0.0.1:2222/gateway) и создай устройство.
   Для подключения используй QR или скачанный `.conf` в AmneziaWG.
   Маршруты настраиваются в [редакторе правил](http://127.0.0.1:2222/routing?tab=singbox).

Для теста WARP положи свой профиль в формате проекта в
`$AWG_PREVIEW_DIR/bootstrap/warp.json` с правами `0600` и перед **первым**
запуском выполни `export AWG_ORIGINAL_WARP_PROFILE_FILE=/bootstrap/warp.json`.
Новая установка без файла использует прямой выход. Профиль WARP и ключи
устройств создаются и хранятся отдельно от образа.

Остановка — `docker compose down`. Данные устройств и правила сохраняются
в томе Compose.
