.PHONY: restart

restart:
	docker-compose down && docker-compose up --build