package dns

type Server struct {
	Name          string `json:"name"`
	ServerAddress string `json:"server_address"`
}

type DNS struct {
	Device struct {
		Name string `json:"name" validate:"required"`
	} `json:"device" validate:"required"`
	ServerList []Server `json:"server_list" validate:"omitempty,dive"`
}
