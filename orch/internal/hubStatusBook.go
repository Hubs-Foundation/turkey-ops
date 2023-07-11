package internal

import "sync"

type hubsStatusBook struct {
	book map[string]hubsStatusInfo
	mu   sync.Mutex
}

type hubsStatusInfo struct {
	HubId  string
	Status string
}

func NewHubsStatusBook() *hubsStatusBook {

	//start polling from rootOrch

	return &hubsStatusBook{
		book: map[string]hubsStatusInfo{},
	}
}

func (hsb *hubsStatusBook) GetStatus(subdomain string) string {
	hsb.mu.Lock()
	defer hsb.mu.Unlock()
	return hsb.book[subdomain].Status
}

func (hsb *hubsStatusBook) PhoneHome(subdomain, status string) {
	Logger.Sugar().Debugf("reporting back: <%v : %v>", subdomain, status)
	//curl back to rootOrch
}
