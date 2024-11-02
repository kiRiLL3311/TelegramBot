package telegram

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"test/Clients/telegram"
	"test/events"
	e "test/lib"
	"test/storage"
)

const (
	off = 5
)

const (
	stateNone                = ""
	stateAwaitingRemoveIndex = "awaiting_remove_index"
)

type Processor struct {
	tg      *telegram.Client
	offset  int
	storage storage.Storage
	states  map[int]string // chatID -> state
	mu      sync.Mutex
}

type Meta struct {
	ChatID   int
	Username string
}

var (
	ErrUnknownEventType = errors.New("unknown event type")
	ErrUnknownMetaType  = errors.New("unlnown meta type")
)

func New(client *telegram.Client, storage storage.Storage) *Processor {
	return &Processor{
		tg:      client,
		storage: storage,
		states:  make(map[int]string),
	}

}

func (p *Processor) Fetch(limit int) ([]events.Event, error) {
	updates, err := p.tg.Updates(p.offset, limit)

	if err != nil {

		updates, _ = p.ImpFetcher(limit)
	}

	if len(updates) == 0 {
		return nil, nil
	}
	// создаем срез с элементами типа events.Event, 0 - длина, len(update) - емкость среза
	res := make([]events.Event, 0, len(updates))

	for _, u := range updates {
		res = append(res, event(u))
	}

	p.offset = updates[len(updates)-1].ID + 1

	return res, nil
}

func (p *Processor) ImpFetcher(limit int) ([]telegram.Update, error) {
	counter := 1

	var err error

	for counter < off {
		log.Printf("its the %d retry", counter)
		_, err = p.tg.Updates(p.offset, limit)
		if err != nil {
			counter++

		} else {
			updates, _ := p.tg.Updates(p.offset, limit)
			return updates, nil
		}
	}

	return nil, e.Wrap("cant get events", err)
}

// }
// func (p *Processor) impFetcher(limit int) ([]telegram.Update, error) {
// 	counter := 0

// 	var err error

// 	for counter < off {
// 		log.Printf("its the %d retry", counter+1)
// 		_, err = p.tg.Updates(p.offset, limit)
// 		if err != nil {
// 			counter++

// 		} else {
// 			updates, _ := p.tg.Updates(p.offset, limit)
// 			return updates, nil
// 		}
// 	}

//		return nil, e.Wrap("cant get events", err)
//	}
func (p *Processor) Process(event events.Event) error {
	switch event.Type {
	case events.Message:
		return p.processMessage(event)
	default:
		return e.Wrap("can't process message", ErrUnknownEventType)
	}
}

func (p *Processor) index(event events.Event, meta Meta) error {
	// Handle remove index
	index, err := strconv.Atoi(strings.TrimSpace(event.Text))
	if err != nil {
		p.tg.SendMessage(meta.ChatID, msgWrongInput)
		return nil
	}

	pages, err := p.storage.ListPrepared(context.Background(), meta.Username)
	if err != nil {
		if errors.Is(err, storage.ErrNoSavedPages) {
			p.tg.SendMessage(meta.ChatID, msgNoSavedPages)
			p.mu.Lock()
			delete(p.states, meta.ChatID)
			p.mu.Unlock()
			return nil
		}
		return err
	}

	if index < 1 || index > len(*pages) {
		p.tg.SendMessage(meta.ChatID, msgWrongInput)
		return nil
	}

	page := (*pages)[index-1]
	element := &storage.Page{
		URL:      page.URL,
		Username: meta.Username,
	}

	if err := p.storage.Remove(context.Background(), element); err != nil {
		return err
	}

	if err := p.tg.SendMessage(meta.ChatID, msgRemoved); err != nil {
		return err
	}

	p.mu.Lock()
	delete(p.states, meta.ChatID)
	p.mu.Unlock()

	return nil
}

func (p *Processor) processMessage(event events.Event) error {
	meta, err := meta(event)
	if err != nil {
		return e.Wrap("cant process message", err)
	}

	p.mu.Lock()
	state, exists := p.states[meta.ChatID]
	p.mu.Unlock()

	if exists && state == stateAwaitingRemoveIndex {
		return p.index(event, meta)
	}

	if err := p.doCmd(event.Text, meta.ChatID, meta.Username); err != nil {
		return e.Wrap("cant process message", err)
	}

	return nil
}

func meta(event events.Event) (Meta, error) {
	//type assertion
	res, ok := event.Meta.(Meta)

	if !ok {
		return Meta{}, e.Wrap("cant get meta", ErrUnknownMetaType)
	}

	return res, nil
}

func event(upd telegram.Update) events.Event {
	updType := fetchType(upd)

	res := events.Event{
		Type: updType,
		Text: fetchText(upd),
	}

	if updType == events.Message {
		res.Meta = Meta{
			ChatID:   upd.Message.Chat.ID,
			Username: upd.Message.From.Username,
		}
	}

	return res
}

func fetchText(upd telegram.Update) string {
	if upd.Message == nil {
		return ""
	}

	return upd.Message.Text
}

func fetchType(upd telegram.Update) events.Type {
	if upd.Message == nil {
		return events.Unknown
	}
	return events.Message
}
