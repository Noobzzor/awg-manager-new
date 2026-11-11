# AWG Manager на Debian

Нужны Debian x86_64, Docker Engine с Compose, устройство `/dev/net/tun` и
свободный входящий UDP-порт 51820. Для подключения из интернета у сервера
должен быть доступный IPv4 или домен.

## Запуск готового образа

Распакуй тестовый комплект и выполни в папке с `compose.yml`:

```sh
docker load -i awg-original-preview-image.tar.gz
export AWG_PREVIEW_IMAGE=awgm-original-preview:clean
export AWG_PREVIEW_DIR="$HOME/.local/share/awgm-original-preview"
export AWG_PANEL_UID="$(id -u)"
export AWG_PANEL_GID="$(id -g)"
export AWG_GATEWAY_PUBLIC_ENDPOINT="192.0.2.10:51820"
install -d -m 700 "$AWG_PREVIEW_DIR/panel" "$AWG_PREVIEW_DIR/bootstrap"
docker compose up -d --no-build
docker compose ps
```

Подставь адрес своего сервера вместо `192.0.2.10`. При выборе другого
внешнего порта задай `AWG_UDP_PORT` и укажи тот же порт в
`AWG_GATEWAY_PUBLIC_ENDPOINT`.

## Панель и подключение

На компьютере, где открываешь панель, запусти SSH-туннель. Подставь имя
SSH-пользователя, адрес и полный путь на сервере к папке из
`AWG_PREVIEW_DIR`. Используй того же пользователя, чей ID указан в
`AWG_PANEL_UID`.

```sh
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:2222:/home/USER/.local/share/awgm-original-preview/panel/awg.sock USER@SERVER
```

Открой [VPS Gateway](http://127.0.0.1:2222/gateway) и создай устройство.
Для подключения используй QR или скачанный `.conf` в AmneziaWG.
Маршруты настраиваются в [редакторе правил](http://127.0.0.1:2222/routing?tab=singbox).

Для теста WARP положи свой профиль в формате проекта в
`$AWG_PREVIEW_DIR/bootstrap/warp.json` с правами `0600` и перед **первым**
запуском выполни `export AWG_ORIGINAL_WARP_PROFILE_FILE=/bootstrap/warp.json`.
Новая установка без файла использует прямой выход.

Остановка — `docker compose down`. Данные устройств и правила сохраняются
в томе Compose.

## Сборка из исходников

Готовый образ можно использовать сразу после импорта. Если нужно изменить код,
сборка из исходников по-прежнему использует старый образ как источник трёх
туннельных бинарников и их лицензий. Финальный образ собирается на чистом Debian.

1. Загрузи `install/image.tar.gz` из архива AWG от 23 октября командой
   `docker load -i install/image.tar.gz`. Тег базы — `awgm-personal-trial:20261021-dns1`.
   Ожидаемый ID — `sha256:2178524bc069afd4aa2c5b639e6acb73154f96a24247db07c33d685fa0b89929`.
2. Из исходников открой `vps-gateway-next/docker/original-preview`. Выполни
   команды из раздела запуска, начиная с `export AWG_PREVIEW_DIR`.
   Перед запуском выполни `unset AWG_PREVIEW_IMAGE` и `docker compose build awg`.
   Для скачивания зависимостей при сборке нужен интернет.
