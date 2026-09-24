# Pantheon OAuth chat bot

Тестовый бот для Public API стримингового сервиса Пантеон. Он читает чат и поддерживает команды:

- `!привет` — отвечает `привет от API`;
- `!title Название` — меняет название стрима. Команда доступна только владельцу
  канала и модераторам по значкам пользователя в чате.

## Настройка

1. Создайте приложение на портале (dev.пантеон.com)[dev.пантеон.com]
2. Укажите в приложении на портале callback URL:
   `http://localhost:1230/callback`.
3. Выдайте приложению scopes `chat:read`, `chat:write`, `streams:manage` и
   `user:read`.
4. В `PANTHEON_CHANNEL_ID` укажите числовой ID профиля владельца нужного чата.
5. Скопируйте `.env.example` в `.env`.
6. Заполните `PANTHEON_CLIENT_ID`, `PANTHEON_CLIENT_SECRET` и числовой
   `PANTHEON_CHANNEL_ID`. Для public-клиента secret оставьте пустым.

## Запуск

```powershell
Copy-Item .env.example .env
go run ./cmd/bot
```

Приложение напечатает ссылку авторизации. Авторизацию должен подтвердить
владелец канала из `PANTHEON_CHANNEL_ID`: изменение стрима выполняется от его
имени. После callback бот начнёт читать live-события чата. Access token и refresh
token хранятся только в памяти процесса.
