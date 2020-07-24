run:
	eval $$(egrep -v '^#' .env | xargs) go run main.go