.PHONY: restart

restart:
	docker-compose down -v --remove-orphans && docker-compose up --build