# 🏴‍☠️ Платформа Capture The Flag (CTF)

Добро пожаловать на **CTF Платформу** – динамическую и автоматизированную систему Capture The Flag, разработанную для кибербезопасностных энтузиастов, преподавателей и организаторов мероприятий! Эта платформа обеспечивает простое управление CTF-соревнованиями с возможностью **автоматического добавления задач**, что гарантирует плавный и увлекательный опыт для участников. 🚀

---

## ✨ Функции

✅ **Автоматическое добавление задач** – больше не нужно загружать задачи вручную! Платформа автоматически добавляет новые задачи, чтобы конкуренция всегда оставалась свежей.  
✅ **Аутентификация пользователей** – безопасная система входа с поддержкой командных и одиночных соревнований.  
✅ **Система подсчета очков** – автоматически вычисляет очки в зависимости от сложности задачи и точности решения.  
✅ **Таблица лидеров в реальном времени** – отслеживайте прогресс участников с помощью интерактивной таблицы лидеров.  
✅ **Задачи в нескольких категориях** – поддерживаются различные типы задач, включая криптографию, веб-эксплуатацию, криминалистику и многое другое!  
✅ **Проверка флагов** – Автоматическая проверка флагов.  
✅ **Развертывание с помощью Docker** – простое и эффективное развертывание с использованием Docker.  
✅ **Панель администратора** – удобный интерфейс для управления задачами, пользователями и настройками соревнований. 

---

## 🚀 Установка

### Предварительные требования

Убедитесь, что у вас установлены следующие компоненты:
- [Docker](https://www.docker.com/) и [Docker Compose](https://docs.docker.com/compose/)
- Mysql для хранения данных

### Шаги

1. **Клонируйте репозиторий**

```bash
git clone https://github.com/Aapng-cmd/ctf_platform.git
cd ctf_platform
```

2. **Настройте окружение**

Отредактируйте `docker-compose.yml` и `init.sql` для настройки учетных данных базы данных.

3. **Запустите платформу (рекомендуется Docker)**

```bash
docker-compose up -d
```

4. **Получите доступ к платформе**

Откройте [http://localhost:80](http://localhost:80) в вашем браузере и начните захватывать флаги! 🏆

---

## 🎯 Использование

- **Администраторы** могут создавать задачи, управлять пользователями и отслеживать прогресс соревнований.
- **Игроки** могут регистрироваться, присоединяться к командам, решать задачи и отправлять флаги для получения очков.
- **Задачи** классифицируются и динамически обновляются на основе заранее заданных правил или внешних источников задач.

---

## 🛠 Конфигурация

Вся конфигурация находится в файлах `docker-compose.yml` и `init.sql`. Настройте их в соответствии с вашим окружением.

---

## 📌 TODO
 
🔹 **Система подсказок** – предоставление игрокам опциональных подсказок для решения задач.  
🔹 **Управление командами** – улучшение функционала команд, включая приглашения в команды, совместную работу и подсчет очков.

Следите за обновлениями! 🚀

---

Счастливого хакинга! 🎉🏴‍☠️🚀
