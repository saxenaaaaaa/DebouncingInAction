# DebouncingInAction

This code implements cache debouncing in go. Debouncing is a technique that is used to save the server from a request spike for a particular key. It means, to let all requests for a particular key wait while the first request for that key is populating a cache during cache miss. This saves DB from getting a request spike for a particular key when all requests see a cache miss in a short duration and try to go to DB and overload it. 

In this implementation simulating a request spike, We see an interesting pattern here : 

When there is a request spike, debouncing increased performance.
But, as soon as I introduced a 50ms time gap between requests, my debouncing solution was  performing slower than the normal no-debounce solution.
In my test simulation, 
 
fetching from DB takes 1 sec
fetching from cache takes 100ms
100 requests are contending for 2 keys (50 req per key)

i.e. with DB fetch being 10 times slower than cache, if requests are 50ms apart, debouncing solution takes ~6 seconds and no-debounce takes ~5 seconds.

Play around here - https://go.dev/play/p/FuNj79Sn0tO

I am guessing the overhead of synchronisation outweighs the benefits of debouncing in this case.

If my test does not have any blunder, this test has really got me questioning what all factors should we consider while going for debouncing and how does it affect normal day latency as opposed to handling a spike day. 
