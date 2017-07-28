/*
   Terminews is a terminal based (TUI) RSS feed manager.
   Copyright (C) 2017  Alexandros Ntavelos, a[dot]ntavelos[at]gmail[dot]com

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/
package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/antavelos/terminews/db"
	c "github.com/jroimartin/gocui"
)

// updateSummary updates the summary View based on the currently selected
// news item
func updateSummary() error {
	summary.Clear()

	currItem := newsList.CurrentItem()
	if currItem == nil {
		return nil
	}
	event := currItem.(db.Event)

	authorLine := fmt.Sprintf("%v %v", Bold.Sprint("By:"), event.Author)
	publishedLine := fmt.Sprintf("%v %v", Bold.Sprint("Published on:"), event.Published)
	urlLine := fmt.Sprintf("%v %v", Bold.Sprint("URL:"), event.Url)

	w, _ := summary.Size()
	summaryLine := strings.Join(JustifiedLines(event.Summary, w-2), "\n ")

	_, err := fmt.Fprintf(summary, "\n\n %v\n %v\n %v\n\n\n %v",
		authorLine, publishedLine, urlLine, Bold.Sprint(summaryLine))

	return err
}

// updateNews updates the news list according to the given events
func updateNews(events []db.Event, from string) error {
	newsList.Reset()
	summary.Clear()

	if len(events) == 0 {
		newsList.SetTitle(fmt.Sprintf("No news in %v", from))
		return nil
	}
	newsList.SetTitle(fmt.Sprintf("News from: %v", from))

	data := make([]interface{}, len(events))
	for i, e := range events {
		data[i] = e
	}

	return newsList.SetItems(data)
}

// loadSites loads the sites from DB and displays them in the list
func loadSites() error {
	sitesList.SetTitle("Sites")

	sites, err := tdb.GetSites()
	if err != nil {
		fmt.Errorf("Failed to load sites: %v", err)
	}
	if len(sites) == 0 {
		sitesList.SetTitle("No sites yet... (Ctrl-n to add)")
		sitesList.Reset()
		newsList.Reset()
		newsList.SetTitle("No news yet...")
		return nil
	}
	data := make([]interface{}, len(sites))
	for i, rr := range sites {
		data[i] = rr
	}

	return sitesList.SetItems(data)
}

// createContentView creates a view where the contents of thecurrently selected
// event will be displayed
func createContentView(g *c.Gui) error {
	tw, th := g.Size()
	v, err := g.SetView(CONTENT_VIEW, tw/8, th/8, (tw*7)/8, (th*7)/8)
	if err != nil && err != c.ErrUnknownView {
		return err
	}
	contentList = CreateList(v, false)
	setTopWindowTitle(g, CONTENT_VIEW, "")
	_, err = g.SetCurrentView(CONTENT_VIEW)

	return err
}

// createPromptView creates a general purpose view to be used as input source
// from the user
func createPromptView(g *c.Gui, title string) error {
	tw, th := g.Size()
	v, err := g.SetView(PROMPT_VIEW, tw/6, (th/2)-1, (tw*5)/6, (th/2)+1)
	if err != nil && err != c.ErrUnknownView {
		return err
	}
	v.Editable = true
	setTopWindowTitle(g, PROMPT_VIEW, title)

	g.Cursor = true
	_, err = g.SetCurrentView(PROMPT_VIEW)

	return err
}

// deleteContentView deletes the current prompt view
func deleteContentView(g *c.Gui) error {
	g.Cursor = false
	return g.DeleteView(CONTENT_VIEW)
}

// deletePromptView deletes the current prompt view
func deletePromptView(g *c.Gui) error {
	g.Cursor = false
	return g.DeleteView(PROMPT_VIEW)
}

func setTopWindowTitle(g *c.Gui, view_name, title string) {
	v, err := g.View(view_name)
	if err != nil {
		log.Println("Error on setTopWindowTitle", err)
		return
	}
	v.Title = fmt.Sprintf("%v (Ctrl-q to close)", title)
}

func isNewSitePrompt(v *c.View) bool {
	return strings.Contains(v.Title, "New site") || strings.Contains(v.Title, "try again")
}

func isFindPrompt(v *c.View) bool {
	return strings.Contains(v.Title, "Search ")
}

// eventSatisfiesSearch searches within thr title and the summary of an event
// and if a list of terms exists conjuctively and case insensitively
func eventSatisfiesSearch(terms []string, e db.Event) bool {
	for _, term := range terms {
		tl := strings.ToLower(term)
		title := strings.ToLower(e.Title)
		summary := strings.ToLower(e.Summary)
		if !strings.Contains(title, tl) && !strings.Contains(summary, tl) {
			return false
		}
	}
	return true
}

// findEvents downloads the events of every available site and returns those
// which match the given terms
func findEvents(terms []string) chan db.Event {
	c := make(chan db.Event)
	sites, err := tdb.GetSites()
	if err != nil {
		close(c)
	}
	go func() {
		for _, site := range sites {
			events, err := DownloadEvents(site.Url)
			if err != nil {
				continue
			}
			for _, e := range events {
				if eventSatisfiesSearch(terms, e) {
					c <- e
				}
			}
		}
		close(c)
	}()
	return c
}

// Key binding functions

func quit(g *c.Gui, v *c.View) error {
	return c.ErrQuit
}

func switchView(g *c.Gui, v *c.View) error {
	g.SelFgColor = c.ColorGreen | c.AttrBold
	if v == sitesList.View {
		newsList.Focus(g)
		sitesList.Unfocus()
		if strings.Contains(newsList.Title, "bookmarks") {
			g.SelFgColor = c.ColorBlue | c.AttrBold
		}
	} else {
		sitesList.Focus(g)
		newsList.Unfocus()
	}
	return nil
}

func listUp(g *c.Gui, v *c.View) error {
	switch v.Name() {

	case SITES_VIEW:
		if err := sitesList.MoveUp(); err != nil {
			log.Println("Error on sitesList.MoveUp()", err)
			return err
		}
	case NEWS_VIEW:
		if err := newsList.MoveUp(); err != nil {
			log.Println("Error on newsList.MoveUp()", err)
			return err
		}
		if err := updateSummary(); err != nil {
			log.Println("Error on updateSummary()", err)
			return err
		}
	case CONTENT_VIEW:
		if err := contentList.MoveUp(); err != nil {
			log.Println("Error on contentList.MoveUp()", err)
			return err
		}
	}
	return nil
}

func listDown(g *c.Gui, v *c.View) error {
	switch v.Name() {

	case SITES_VIEW:
		if err := sitesList.MoveDown(); err != nil {
			log.Println("Error on sitesList.MoveDown()", err)
			return err
		}
	case NEWS_VIEW:
		if err := newsList.MoveDown(); err != nil {
			log.Println("Error on newsList.MoveDown()", err)
			return err
		}
		if err := updateSummary(); err != nil {
			log.Println("Error on updateSummary()", err)
			return err
		}
	case CONTENT_VIEW:
		if err := contentList.MoveDown(); err != nil {
			log.Println("Error on contentList.MoveDown()", err)
			return err
		}
	}
	return nil
}

func listPgDown(g *c.Gui, v *c.View) error {
	switch v.Name() {

	case SITES_VIEW:
		if err := sitesList.MovePgDown(); err != nil {
			log.Println("Error on sitesList.MovePgDown()", err)
			return err
		}
	case NEWS_VIEW:
		if err := newsList.MovePgDown(); err != nil {
			log.Println("Error on newsList.MovePgDown()", err)
			return err
		}
		if err := updateSummary(); err != nil {
			log.Println("Error on updateSummary()", err)
			return err
		}
	case CONTENT_VIEW:
		if err := contentList.MovePgDown(); err != nil {
			log.Println("Error on contentList.MovePgDown()", err)
			return err
		}
	}
	return nil
}

func listPgUp(g *c.Gui, v *c.View) error {
	switch v.Name() {

	case SITES_VIEW:
		if err := sitesList.MovePgUp(); err != nil {
			log.Println("Error on sitesList.MovePgUp()", err)
			return err
		}
	case NEWS_VIEW:
		if err := newsList.MovePgUp(); err != nil {
			log.Println("Error on newsList.MovePgUp()", err)
			return err
		}
		if err := updateSummary(); err != nil {
			log.Println("Error on updateSummary()", err)
			return err
		}
	case CONTENT_VIEW:
		if err := contentList.MovePgUp(); err != nil {
			log.Println("Error on contentList.MovePgUp()", err)
			return err
		}
	}
	return nil
}

func onEnter(g *c.Gui, v *c.View) error {
	switch v.Name() {
	case SITES_VIEW:
		currItem := sitesList.CurrentItem()
		if currItem == nil {
			return nil
		}
		site := currItem.(db.Site)

		summary.Clear()
		newsList.Clear()
		newsList.Focus(g)
		g.SelFgColor = c.ColorGreen | c.AttrBold
		newsList.Title = " Fetching ... "
		g.Execute(func(g *c.Gui) error {
			events, err := DownloadEvents(site.Url)
			if err != nil {
				newsList.Title = fmt.Sprintf(" Failed to load news from: %v ", site.Name)
				newsList.Clear()
			} else {
				newsList.Focus(g)
				if err := updateNews(events, site.Name); err != nil {
					log.Println("Error on updateNews", err)
					return err
				}
				if err := updateSummary(); err != nil {
					log.Println("Error on updateSummary", err)
					return err
				}
			}
			return nil
		})
	case PROMPT_VIEW:
		if isNewSitePrompt(v) {
			url := strings.TrimSpace(v.ViewBuffer())
			if len(url) == 0 {
				return nil
			}
			g.Execute(func(g *c.Gui) error {
				feed, err := CheckUrl(url)
				if err != nil {
					setTopWindowTitle(g, PROMPT_VIEW, "Invalid URL, try again:")
					g.SelFgColor = c.ColorRed | c.AttrBold
					return nil
				}

				_, err = tdb.GetSiteByUrl(url)
				if err != nil {
					if _, ok := err.(db.NotFound); !ok {
						setTopWindowTitle(g, PROMPT_VIEW, "Site already exists, try again:")
						g.SelFgColor = c.ColorRed | c.AttrBold
						return nil
					}
				} else {
					log.Println("Error o GetSiteByUrl", err)
					return err
				}

				rr := db.Site{Name: feed.Title, Url: url}
				if err := tdb.AddSite(rr); err != nil {
					log.Println("Error on AddSite", err)
					return err
				}
				deletePromptView(g)
				g.SelFgColor = c.ColorGreen | c.AttrBold
				sitesList.Focus(g)

				if err = loadSites(); err != nil {
					log.Println("Error on loadSites", err)
					return err
				}

				return nil
			})
		}
		if isFindPrompt(v) {
			newsList.Reset()
			newsList.Focus(g)
			sitesList.Unfocus()
			newsList.Title = " Searching ... "
			deletePromptView(g)
			terms := strings.Split(strings.TrimSpace(v.ViewBuffer()), " ")
			g.Execute(func(g *c.Gui) error {
				c := 0
				for event := range findEvents(terms) {
					newsList.AddItem(g, event)
					c++
				}
				if c == 0 {
					newsList.SetTitle("No events found")
				} else {
					newsList.SetTitle(fmt.Sprintf("%v event(s) found", c))
				}
				return nil
			})
		}
	}

	return nil
}

func addBookmark(g *c.Gui, v *c.View) error {
	if v.Name() == NEWS_VIEW {
		currItem := newsList.CurrentItem()
		if currItem == nil {
			return nil
		}
		event := currItem.(db.Event)
		if err := tdb.AddEvent(event); err != nil {
			log.Println("Error on AddEvent", err)
			return err
		}
	}
	return nil
}

func loadBookmarks(g *c.Gui, v *c.View) error {
	events, err := tdb.GetEvents()
	if err != nil {
		log.Println("Error on AddEvent", err)
		return err
	}
	source := "My bookmarks"
	if err != nil {
		newsList.Title = fmt.Sprintf(" Failed to load news from: %v ", source)
		newsList.Clear()
	} else {
		newsList.Focus(g)
		if err := updateNews(events, source); err != nil {
			log.Println("Error on updateNews", err)
			return err
		}
		if err := updateSummary(); err != nil {
			log.Println("Error on updateSummary", err)
			return err
		}
	}
	g.SelFgColor = c.ColorBlue | c.AttrBold
	return nil
}

func deleteEntry(g *c.Gui, v *c.View) error {
	if v == sitesList.View {
		currItem := sitesList.CurrentItem()
		if currItem == nil {
			return nil
		}
		rr := currItem.(db.Site)
		if err := tdb.DeleteSite(rr.Id); err != nil {
			log.Println("Error on DeleteSite", err)
			return err
		}
		if err := loadSites(); err != nil {
			log.Println("Error on loadSites", err)
			return err
		}
	} else {
		if strings.Contains(newsList.Title, "My bookmarks") {
			currItem := newsList.CurrentItem()
			if currItem == nil {
				return nil
			}
			event := currItem.(db.Event)
			if err := tdb.DeleteEvent(event.Id); err != nil {
				log.Println("Error on DeleteEvent", err)
				return err
			}
			if err := loadBookmarks(g, v); err != nil {
				log.Println("Error on loadBookmarks", err)
				return err
			}
		}
	}
	return nil
}

func removeTopView(g *c.Gui, v *c.View) error {
	switch v.Name() {

	case PROMPT_VIEW:
		sitesList.Focus(g)
		g.SelFgColor = c.ColorGreen | c.AttrBold
		if err := deletePromptView(g); err != nil {
			log.Println("Error on deletePromptView", err)
			return err
		}
	case CONTENT_VIEW:
		newsList.Focus(g)
		if err := deleteContentView(g); err != nil {
			log.Println("Error on deleteContentView", err)
			return err
		}
	}
	return nil
}

func addSite(g *c.Gui, v *c.View) error {
	if err := createPromptView(g, "New site URL:"); err != nil {
		log.Println("Error on createPromptView", err)
		return err
	}

	return nil
}

func find(g *c.Gui, v *c.View) error {
	if err := createPromptView(g, "Search with multiple terms:"); err != nil {
		log.Println("Error on createPromptView", err)
		return err
	}

	return nil
}

func loadContent(g *c.Gui, v *c.View) error {
	if v.Name() == NEWS_VIEW {
		if err := createContentView(g); err != nil {
			log.Println("Error on createContentView", err)
			return err
		}
		cv, _ := g.View(CONTENT_VIEW)
		cv.Title = "Fetching..."
		g.Execute(func(g *c.Gui) error {
			contentList.Focus(g)
			currItem := newsList.CurrentItem()
			if currItem == nil {
				return nil
			}
			event := currItem.(db.Event)
			CurrentContent = GetContent(event.Url)
			if err := UpdateContent(g, CurrentContent); err != nil {
				log.Println("Error on UpdateContent", err)
				return err
			}
			contentList.SetTitle(fmt.Sprintf("%v (Ctrl-q to close)", event.Title))
			return nil
		})

	}
	return nil
}

func UpdateContent(g *c.Gui, content []string) error {
	w, _ := contentList.Size()
	contentList.AddItem(g, "")
	for _, text := range content {
		lines := JustifiedLines(text, w-2)
		for _, l := range lines {
			err := contentList.AddItem(g, l)
			if err != nil {
				log.Println("Error on contentList.AddItem", err)
				return err
			}
		}
		contentList.AddItem(g, "")
	}
	return nil
}

func OpenBrowser(g *c.Gui, v *c.View) error {
	currItem := newsList.CurrentItem()
	if currItem == nil {
		return nil
	}
	event := currItem.(db.Event)
	if v.Name() == NEWS_VIEW {
		cmd := exec.Command("xdg-open", event.Url)

		if err := cmd.Run(); err != nil {
			log.Println("Error on opening browser", err)
			return err
		}
	}
	return nil
}
