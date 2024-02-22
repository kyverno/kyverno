{{ define "packages" }}
  <html lang="en">
    <head>
      <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.3.1/css/bootstrap.min.css">
<style>
    .bg-blue {
        color: #ffffff;
        background-color: #1589dd;
    }
</style>
    </head>
    <body>
      <div class="container">
        {{ range .packages }}
          {{/* Only display package that has a group name */}}
          {{ if ne .GroupName "" }}
            <h2 id="{{- .Anchor -}}">Package: <span style="font-family: monospace">{{- .DisplayName -}}</span></h2>
            <p>{{ .GetComment }}</p>
          {{ end }}
        {{ end }}
        {{ range .packages }}
          {{ if ne .GroupName "" }}
            {{/* TODO: Make the following line conditional */}}
            <h3>Resource Types:</h3>
            <ul>
              {{- range .VisibleTypes -}}
                {{ if .IsExported -}}
                  <li>
                    <a href="{{ .Link }}">{{ .DisplayName }}</a>
                  </li>
                {{- end }}
              {{- end -}}
            </ul>

            {{/* For package with a group name, list all type definitions in it. */}}
            {{ range .VisibleTypes }}
              {{- if or .Referenced .IsExported -}}
                {{ template "type" .  }}
              {{- end -}}
            {{ end }}
          {{ else }}
            {{/* For package without a group name, list only type definitions that are referenced. */}}
            {{ range .VisibleTypes }}
              {{ if .Referenced }}
                {{ template "type" . }}
              {{ end }}
            {{ end }}
          {{ end }}
          <hr />
        {{ end }}
      </div>
    </body>
  </html>
{{ end }}
