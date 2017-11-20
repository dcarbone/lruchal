test:
	go test -v && go test -race && go test -run=_nothing_ -bench=.