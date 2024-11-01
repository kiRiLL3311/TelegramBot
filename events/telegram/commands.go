package telegram

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	e "test/lib"
	"test/storage"
)

const (
	RndCmd   = "/rnd"
	HelpCmd  = "/help"
	StartCmd = "/start"
	ListCmd  = "/list"
	RemCmd   = "/remove"
)

func (p *Processor) doCmd(text string, chatID int, username string) error {
	text = strings.TrimSpace(text)

	log.Printf("got new command '%s' from '%s'", text, username)

	if isAddCmd(text) {
		return p.savePage(chatID, text, username)
	}

	switch text {
	case RndCmd:
		return p.sendRandom(chatID, username)
	case RemCmd:
		return p.sendRemove(username, chatID)
	case HelpCmd:
		return p.sendHelp(chatID)
	case StartCmd:
		return p.sendHello(chatID)
	case ListCmd:
		return p.sendList(chatID, username)
	default:
		return p.tg.SendMessage(chatID, msgUnknownCommand)
	}

}

func (p *Processor) savePage(chatID int, pageURL string, username string) (err error) {
	defer func() { err = e.WrapIfErr("cant do command: save page", err) }()

	if p.storage == nil {
		return fmt.Errorf("storage is not initialized")
	}
	if p.tg == nil {
		return fmt.Errorf("tg is not initialized")
	}

	page := &storage.Page{
		URL:      pageURL,
		Username: username,
	}

	isExists, err := p.storage.IsExists(page)
	if err != nil {
		return err
	}

	if isExists {
		if err := p.tg.SendMessage(chatID, msgAlreadyExists); err != nil {
			return err
		}
		return nil
	}

	if err := p.storage.Save(page); err != nil {
		return err
	}

	if err := p.tg.SendMessage(chatID, msgSaved); err != nil {
		return err
	}

	return nil
}

func (p *Processor) sendRemove(username string, chatID int) error {
	pages, err := p.storage.ListPrepared(username)
	if err != nil {
		if errors.Is(err, storage.ErrNoSavedPages) {
			return p.tg.SendMessage(chatID, msgNoSavedPages)
		}
		return err
	}

	message := formatPages(pages) + "\n" + msgInsertNumber

	if err := p.tg.SendMessage(chatID, message); err != nil {
		return err
	}
	if errors.Is(err, storage.ErrNoSavedPages) {
		return p.tg.SendMessage(chatID, msgNoSavedPages)
	}

	// Set the user's state to await index input
	p.mu.Lock()
	p.states[chatID] = stateAwaitingRemoveIndex
	p.mu.Unlock()

	return nil
}

func (p *Processor) sendRandom(chatID int, username string) (err error) {
	defer func() { err = e.WrapIfErr("can't do command: can't send random", err) }()

	page, err := p.storage.PickRandom(username)
	if err != nil && !errors.Is(err, storage.ErrNoSavedPages) {
		return err
	}
	if errors.Is(err, storage.ErrNoSavedPages) {
		return p.tg.SendMessage(chatID, msgNoSavedPages)
	}

	if err := p.tg.SendMessage(chatID, page.URL); err != nil {
		return err
	}

	return p.storage.Remove(page)
}
func formatPages(pages *[]storage.Page) string {
	var result strings.Builder
	i := 0
	for _, page := range *pages {
		result.WriteString(strconv.Itoa(i+1) + ")")
		result.WriteString(page.URL)
		result.WriteString("\n") // Добавляем новую строку для разделения ссылок
		i++
	}
	return result.String()
}

func (p *Processor) sendList(chatID int, username string) error {

	pages, err := p.storage.ListPrepared(username)
	if err != nil && !errors.Is(err, storage.ErrNoSavedPages) {
		return err
	}
	if errors.Is(err, storage.ErrNoSavedPages) {
		return p.tg.SendMessage(chatID, msgNoSavedPages)
	}

	message := formatPages(pages)

	// Sending list of links
	if err := p.tg.SendMessage(chatID, message); err != nil {
		return err
	}

	return nil

}

func (p *Processor) sendHelp(chatID int) error {
	return p.tg.SendMessage(chatID, msgHelp)
}

func (p *Processor) sendHello(chatID int) error {
	return p.tg.SendMessage(chatID, msgHello)
}

func isAddCmd(text string) bool {
	return isURL(text)
}

func isURL(text string) bool {
	u, err := url.Parse(text)

	return err == nil && u.Host != ""
}
