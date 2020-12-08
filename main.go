package main

import (
	"encoding/json"
	"strings"
	"fmt"
	"io/ioutil"
	"time"
	"log"
	"os"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type periodType string
const (
	planTypeEveryNDays periodType = "N"
	planTypeEveryWeekday periodType = "W"
	planTypeOnce periodType = "O"
)

type planStruct struct {
	ChannelId int64
	Id int64
	Period periodType
	PeriodValue string
	LastSent time.Time
	Summary string
}

type plansFileStruct struct {
	plans []planStruct
}

func shouldBeDisplayed(plan planStruct) bool {
	today := getToday()
	if plan.LastSent.Equal(today) {
		return false
	}
	switch plan.Period {
	case planTypeEveryNDays:
		if (plan.LastSent == time.Time{}) {
			return true
		}
		intPeriodValue, err := strconv.ParseInt(plan.PeriodValue, 10, 32)
		if err != nil {
			return false
		}
		durationSinceLastSent := today.Sub(plan.LastSent)
		hoursPassedSinceLastSent := durationSinceLastSent.Hours()
		daysPassedSinceLastSent := int64(hoursPassedSinceLastSent / 24)
		return daysPassedSinceLastSent >= intPeriodValue
	case planTypeEveryWeekday:
		weekday := int64(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		day := strconv.FormatInt(weekday, 10)
		return strings.Contains(plan.PeriodValue, day)
	case planTypeOnce:
		datePeriodValue, err := time.ParseInLocation("02/01/2006", plan.PeriodValue, today.Location())
		if err != nil {
			return false
		}
		return datePeriodValue.Equal(today)
	}
	return false
}

func getToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func sendPlans(bot *tgbotapi.BotAPI) {
	message := []string{}
	plansChannel := parsePlansFile()
	var ChannelId int64
	number := 1

	today := getToday()

	var plans []planStruct

	for plan := range plansChannel {
		ChannelId = plan.ChannelId
		if shouldBeDisplayed(plan) {
			message = append(message, fmt.Sprintf("%v. %s", number, plan.Summary))
			number = number + 1
			plan.LastSent = today
		}
		plans = append(plans, plan)
	}


	if ChannelId == 0 {
		return
	}
	if len(message) == 0 {
		message = append(message, "Планов на сегодня нет")
	} else {
		message = append([]string{"Планы на сегодня:"}, message...)
	}
	bot.Send(tgbotapi.NewMessage(ChannelId, strings.Join(message, "\n")))

	plansJson, err := json.MarshalIndent(plans, "", "    ")
	if err != nil {
		panic(err)
	}

	ioutil.WriteFile("plans.json", plansJson, os.ModeExclusive)
}

func parsePlansFile() chan planStruct {
	result := make(chan planStruct)
	go func () {
		defer close(result)
		var plans []planStruct
		plansBytes, err := ioutil.ReadFile("plans.json")
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(plansBytes, &plans)
		if err != nil {
			panic(err)
		}
		for _, plan := range plans {
			log.Print(plan)
			result <- plan
		}
	}()
	return result
}

func sendPlansEveryDay(bot *tgbotapi.BotAPI) {
	const timeWhenSent = "08h00m"
	whenSent, _ := time.ParseDuration(timeWhenSent)

	today := getToday()
	now := time.Now()
	var baseDate time.Time
	if now.Sub(today) < whenSent {
		baseDate = today
	} else {
		baseDate = today.AddDate(0, 0, 1)
	}

	durationToWait := baseDate.Add(whenSent).Sub(now)

	c := make(chan time.Time)
	go func (c chan time.Time) {
		c <- <- time.After(durationToWait)
		for v := range time.Tick(24 * time.Hour) {
			c <- v
		}
	}(c)

	for _ = range c {
		sendPlans(bot)
	}
}

func main()  {
	bot, err := tgbotapi.NewBotAPI("<PutYourAPIKeyHere>")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	go sendPlansEveryDay(bot)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		// TODO: Add logic to manage tasks
		// msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		// msg.ReplyToMessageID = update.Message.MessageID

		// bot.Send(msg)
	}
}