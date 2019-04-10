# kubetail

**v1.0.0 Released**
https://github.com/jbvmio/kubetail/releases

kubetail is a utility written in go enabling a straight forward way to immediately begin tailing logs from multiple Kubernetes pods using whitelist, blacklist terms if desired. Searches and returns results using podnames as a wildcard search, optionally adding a prefixed header.

All pods that contain the search terms as a substring will be returned.

**Examples:**
```
  kubetail -i apache nginx                 // Tails logs from pods containing "apache" or "nginx", adding an id header.
  kubetail -i pod1 pod2 --tail 20          // Tails pod1 and pod2 beginning with the last 20 lines.
  kubetail --in-cluster pod1               // Use --in-cluster flag if running within a Pod itself.
```

**Using white-list (--grep) and black-list (--vgrep) filters:**
```
  kubetail -i apache nginx --vgrep 'GET,connection refused'
  kubetail -i apache nginx --grep "example.com,mysite.com" --vgrep POST
```
  The latter will tail logs from any pod with "apache" or "nginx" in it's name, filtering for anything containing
  either example.com or mysite.com but not containing the word POST.

**Filter order is determined by the order entered on the command line:**
```
   kubetail -i apache nginx --grep example.com --vgrep POST  // Filters for lines containing example.com first, then filters out any lines containing POST
   kubetail -i apache nginx --vgrep POST --grep example.com  // Filters out any lines containing POST first, then filters for lines containing example.com
```

**Flags:**
```
      --in-cluster      enables kubetail to be used inside a pod.
  -i, --id              display the pod name as a header along with the output.
      --grep strings    only display lines matching the specified text. Use a comma seperated string for multiple args.
      --vgrep strings   exclude any lines matching the specified text. Use a comma seperated string for multiple args.
      --tail int        start tail with defined no. of lines. (default 2)
  -h, --help            help for kubetail
```
