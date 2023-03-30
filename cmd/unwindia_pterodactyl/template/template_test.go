package template

import (
	"github.com/GSH-LAN/Unwindia_common/src/go/matchservice"
	"testing"
)

const (
	templateText1 = `This match is managed by UNWINDIA

{{if not (and .Team1.Ready .Team2.Ready) -}}

As soon both teams are ready your gameserver will be genereated.
{{ else -}} 
{{ if ne .ServerAddress "" -}}
Your server is ready, find the connection details below:

IP-Address: {{ .ServerAddress }}
Password: {{ .ServerPassword }}
RCON-Password: {{ .ServerPasswordMgmt }}

<a href="steam://connect/{{ .ServerAddress -}}/{{ .ServerPassword -}}">connect {{ .ServerAddress -}};password {{ .ServerPassword -}}</a>
{{ else -}} 
Since both Teams are ready now your server is getting prepared. This process will take a few minutes.
{{ end -}}
{{ end -}}
`

	expectedTemplateText1NewMatch = `This match is managed by UNWINDIA

As soon both teams are ready your gameserver will be genereated.
`

	expectedTemplateText1TeamsReady = `This match is managed by UNWINDIA

Since both Teams are ready now your server is getting prepared. This process will take a few minutes.
`

	expectedTemplateText1TeamsAndServerReady = `This match is managed by UNWINDIA

Your server is ready, find the connection details below:

IP-Address: 127.0.0.1:27015
Password: password
RCON-Password: secret

<a href="steam://connect/127.0.0.1:27015/password">connect 127.0.0.1:27015;password password</a>
`

	templateTextBroken = ` This is a broken template {{ .UnKnownAttribute}} `
)

var matchNew = matchservice.MatchInfo{
	Id:   "123",
	MsID: "456",
	Team1: matchservice.Team{
		Id:      "team1id",
		Name:    "cool-team",
		Players: nil,
		Picture: nil,
		Ready:   false,
	},
	Team2: matchservice.Team{
		Id:      "team2id",
		Name:    "nice-teams",
		Players: nil,
		Picture: nil,
		Ready:   false,
	},
	PlayerAmount:       10,
	Game:               "csgo",
	Map:                "",
	ServerAddress:      "",
	ServerPassword:     "",
	ServerPasswordMgmt: "",
	ServerTvAddress:    "",
	ServerTvPassword:   "",
}

var matchTeamsReady = matchservice.MatchInfo{
	Id:   "123",
	MsID: "456",
	Team1: matchservice.Team{
		Id:      "team1id",
		Name:    "cool-team",
		Players: nil,
		Picture: nil,
		Ready:   true,
	},
	Team2: matchservice.Team{
		Id:      "team2id",
		Name:    "nice-teams",
		Players: nil,
		Picture: nil,
		Ready:   true,
	},
	PlayerAmount:       10,
	Game:               "csgo",
	Map:                "",
	ServerAddress:      "",
	ServerPassword:     "",
	ServerPasswordMgmt: "",
	ServerTvAddress:    "",
	ServerTvPassword:   "",
}

var matchTeamsAndServerReady = matchservice.MatchInfo{
	Id:   "123",
	MsID: "456",
	Team1: matchservice.Team{
		Id:      "team1id",
		Name:    "cool-team",
		Players: nil,
		Picture: nil,
		Ready:   true,
	},
	Team2: matchservice.Team{
		Id:      "team2id",
		Name:    "nice-teams",
		Players: nil,
		Picture: nil,
		Ready:   true,
	},
	PlayerAmount:       10,
	Game:               "csgo",
	Map:                "",
	ServerAddress:      "127.0.0.1:27015",
	ServerPassword:     "password",
	ServerPasswordMgmt: "secret",
	ServerTvAddress:    "127.0.0.1:27115",
	ServerTvPassword:   "tvpassword",
}

func TestParseTemplateForMatch(t *testing.T) {
	type args struct {
		tpl       string
		matchinfo *matchservice.MatchInfo
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "ok-template_new_match",
			args: args{
				tpl:       templateText1,
				matchinfo: &matchNew,
			},
			want:    expectedTemplateText1NewMatch,
			wantErr: false,
		},
		{
			name: "ok-template_teams_ready",
			args: args{
				tpl:       templateText1,
				matchinfo: &matchTeamsReady,
			},
			want:    expectedTemplateText1TeamsReady,
			wantErr: false,
		},
		{
			name: "ok-template_teams_and_server_ready",
			args: args{
				tpl:       templateText1,
				matchinfo: &matchTeamsAndServerReady,
			},
			want:    expectedTemplateText1TeamsAndServerReady,
			wantErr: false,
		},
		{
			name: "err-template_text_broken",
			args: args{
				tpl:       templateTextBroken,
				matchinfo: &matchNew,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemplateForMatch(tt.args.tpl, tt.args.matchinfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTemplateForMatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTemplateForMatch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
