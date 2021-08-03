package utils

const dummy = `
<!--  -->
{{if .ShowStackInfo}}
<div class="container-fluid" style="margin-top:3%">
  <table class="table table-striped">
	<!-- <thead><tr><th>stack name</th><th>time started</th><th>last status</th></tr></thead> -->
	<tbody>
	{{range .stacks}}
	<tr><td><a href="{{.stackLink}}">{{.stackName}}</a></td><td>{{.timeStart}}</td><td>{{.lastStatus}}</td></tr>
	{{end}}
	</tbody>
  </table>
  </div>
{{end}}  
<!--  -->
`
