docker:
	exec docker build -t adiprerepa/cost-analyzer-mutating-webhook:0.1.1 -t adiprerepa/cost-analyzer-mutating-webhook:latest -f Dockerfile.mutating-webhook .
	exec docker build -t adiprerepa/cost-analyzer-mutating-webhook-ca:0.1.1 -t adiprerepa/cost-analyzer-mutating-webhook-ca:latest -f Dockerfile.mutating-webhook-ca .
	docker push adiprerepa/cost-analyzer-mutating-webhook:latest
	docker push adiprerepa/cost-analyzer-mutating-webhook-ca:latest

build:
	docker build -t mutating-webhook:0.1.1 -t mutating-webhook:latest -f Dockerfile.mutating-webhook .
	docker build -t mutating-webhook-ca:0.1.1 -t mutating-webhook-ca:latest -f Dockerfile.mutating-webhook-ca .

push:
	docker push adiprerepa/cost-analyzer-mutating-webhook:latest
	docker push adiprerepa/cost-analyzer-mutating-webhook-ca:latest

