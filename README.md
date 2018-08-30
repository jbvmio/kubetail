# kubetail

**v0.7 Released**
https://github.com/jbvmio/kubetail/releases

kubetail is a utility written in go enabling a straight forward way to immediately begin tailing logs from multiple Kubernetes pods using whitelist, blacklist terms if desired. Searches and returns results using podnames as a wildcard search, optionally adding a prefixed header.

All pods that contain the search terms as a substring will be returned.

**Examples:**
```
  kubetail -i apache nginx                 // Tails logs from pods containing "apache" or "nginx", adding an id header.
  kubetail -i pod1 pod2 --tail-lines 20    // Tails pod1 and pod2 beginning with the last 20 lines.
  kubetail --in-cluster pod1               // Use --in-cluster flag if running within a Pod itself.
```

**Using white-list and black-list filters:**
```
  kubetail -i apache nginx -w "example.com,mysite.com" -b POST
```
  This will tail logs from any pod with "apache" or "nginx" in it's name, filtering for anything containing
  either example.com or mysite.com but not containing the word POST.

**Flags:**
```
  -b, --black-list strings   exclude any lines matching the specified text. Use a comma seperated string for multiple args.
  -h, --help                 help for kubetail
  -i, --id                   display the pod name as a header along with the output.
  -k, --k8s                  enables kubetail to be used inside a pod.
  -t, --tail-lines int       start tail with defined no. of lines. (default 10)
  -w, --white-list strings   only display lines matching the specified text. Use a comma seperated string for multiple args.
```

This was written for functionality and troubleshooting purposes, testing out the [channelrouter](https://github.com/jbvmio/channelrouter) package and mostly for fun.

