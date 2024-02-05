{{ define "type" }}
  <H3 id="{{ .Anchor }}">
    {{- .Name.Name }}
    {{ if eq .Kind "Alias" }}(<code>{{ .Underlying }}</code> alias)</p>{{ end -}}
  </H3>

  {{ with .References }}
    <p>
      (<em>Appears in:</em>
      {{- $prev := "" -}}
      {{- range . -}}
        {{- if or .Referenced .IsExported -}}
        {{- if $prev -}}, {{ end -}}
        {{ $prev = . }}
        <a href="{{ .Link }}">{{ .DisplayName }}</a>
        {{- end }}
      {{- end -}}
      )
    </p>
  {{ end }}

  <p>{{ .GetComment }}</p>

  {{ if .GetMembers }}
    <table class="table table-striped">
      <thead class="thead-dark">
        <tr>
          <th>Field</th>
          <th>Description</th>
        </tr>
      </thead>
      <tbody>
        {{/* . is a apiType */}}
        {{ if .IsExported }}
          {{/* Add apiVersion and kind rows if deemed necessary */}}
          <tr>
            <td><code>apiVersion</code></br>string</td>
            <td><code>{{ .APIGroup }}</code></td>
          </tr>
          <tr>
            <td><code>kind</code></br>string</td>
            <td><code>{{ .Name.Name }}</code></td>
          </tr>
        {{ end }}

        {{/* The actual list of members is in the following template */}}
        {{ template "members" .}}

      </tbody>
    </table>
  {{ end }}
{{ end }}
