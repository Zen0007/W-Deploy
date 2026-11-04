MODE ?= local
COMPOSE := docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.$(MODE).yml

.PHONY: up down clean setup logs ps restart

setup:
	INGRESS_MODE=$(MODE) ./scripts/setup.sh

up:
	INGRESS_MODE=$(MODE) ./scripts/setup.sh
	$(COMPOSE) up -d --build
	@echo ""
	@echo "→ Platform : $$(grep '^APP_URL=' .env | cut -d= -f2-)"
	@echo "→ Mode     : $(MODE)"

down:
	$(COMPOSE) down

clean:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=100

ps:
	$(COMPOSE) ps

restart: down up