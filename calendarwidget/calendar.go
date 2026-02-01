package calendar

import (
	"bot/telegram"
	"fmt"
	"strconv"
	"time"
)

func NewCalendar(opt Options) *Calendar {
	if opt.YearRange == [2]int{0, 0} {
		opt.YearRange = [2]int{MinYearLimit, MaxYearLimit}
	}
	if opt.InitialYear == 0 {
		opt.InitialYear = time.Now().Year()
	}
	if opt.InitialMonth == 0 {
		opt.InitialMonth = time.Now().Month()
	}
	if err := opt.validate(); err != nil {
		panic(err)
	}

	return &Calendar{
		kb:        make([][]telegram.InlineKeyboardButton, 0),
		opt:       &opt,
		currYear:  opt.InitialYear,
		currMonth: opt.InitialMonth,
		view:      "cal",
	}
}

type Calendar struct {
	opt       *Options
	kb        [][]telegram.InlineKeyboardButton
	currYear  int
	currMonth time.Month
	view      string // "cal" или "month_pick"
}

// Options represents a struct for passing optional
// properties for customizing a calendar keyboard
type Options struct {
	// The year that will be initially active in the calendar.
	// Default value - today's year
	InitialYear int

	// The month that will be initially active in the calendar
	// Default value - today's month
	InitialMonth time.Month

	// The range of displayed years
	// Default value - {1970, 292277026596} (time.Unix years range)
	YearRange [2]int

	// The language of all designations.
	// If equals "ru" the designations would be Russian,
	// otherwise - English
	Language string
}

// GetKeyboard builds the calendar inline-keyboard
func (cal *Calendar) GetKeyboard() [][]telegram.InlineKeyboardButton {
	cal.clearKeyboard()

	cal.addMonthYearRow()
	cal.addWeekdaysRow()
	cal.addDaysRows()
	cal.addControlButtonsRow()

	return cal.kb
}

// Clears the calendar's keyboard
func (cal *Calendar) clearKeyboard() {
	cal.kb = make([][]telegram.InlineKeyboardButton, 0)
}

// Builds a full row width button with a displayed month's name
// The button represents a list of all months when clicked
func (cal *Calendar) addMonthYearRow() {
	var row []telegram.InlineKeyboardButton

	btn := telegram.InlineKeyboardButton{
		Text:         fmt.Sprintf("%s %v", cal.getMonthDisplayName(cal.currMonth), cal.currYear),
		CallbackData: "cal:view:months", // нажали — показать выбор месяца
	}

	row = append(row, btn)
	cal.addRowToKeyboard(&row)
}

// Builds a keyboard with a list of months to pick
func (cal *Calendar) getMonthPickKeyboard() [][]telegram.InlineKeyboardButton {
	cal.clearKeyboard()

	var row []telegram.InlineKeyboardButton

	// Генерация 12 месяцев (2 колонки)
	for i := 1; i <= 12; i++ {
		monthName := cal.getMonthDisplayName(time.Month(i))

		monthBtn := telegram.InlineKeyboardButton{
			Text:         monthName,
			CallbackData: "cal:month:" + strconv.Itoa(i),
		}

		row = append(row, monthBtn)

		// 2 колонки
		if i%2 == 0 {
			cal.addRowToKeyboard(&row)
			row = []telegram.InlineKeyboardButton{}
		}
	}

	// если вдруг остался неполный ряд (на всякий)
	if len(row) > 0 {
		cal.addRowToKeyboard(&row)
	}

	return cal.kb
}

// Builds a row of non-clickable buttons
// that display weekdays names
func (cal *Calendar) addWeekdaysRow() {
	var row []telegram.InlineKeyboardButton

	for _, wd := range cal.getWeekdaysDisplayArray() {
		btn := telegram.InlineKeyboardButton{
			Text:         wd,
			CallbackData: "cal:noop",
		}
		row = append(row, btn)
	}

	cal.addRowToKeyboard(&row)
}

// Builds a table of clickable cells (buttons) - active month's days
func (cal *Calendar) addDaysRows() {
	beginningOfMonth := time.Date(cal.currYear, cal.currMonth, 1, 0, 0, 0, 0, time.UTC)
	amountOfDaysInMonth := beginningOfMonth.AddDate(0, 1, -1).Day()

	var row []telegram.InlineKeyboardButton

	// Сколько пустых кнопок вставить в начале месяца
	weekdayNumber := int(beginningOfMonth.Weekday())
	if weekdayNumber == 0 && cal.opt.Language == RussianLangAbbr { // русское исключение для воскресенья
		weekdayNumber = 7
	}

	// Разница порядка дней недели en/ru
	if cal.opt.Language != RussianLangAbbr {
		weekdayNumber++
	}

	// Пустые кнопки в начале
	for i := 1; i < weekdayNumber; i++ {
		cal.addEmptyCell(&row)
	}

	// Кнопки дней
	for i := 1; i <= amountOfDaysInMonth; i++ {
		dayText := strconv.Itoa(i)

		cell := telegram.InlineKeyboardButton{
			Text:         dayText,
			CallbackData: "cal:day:" + dayText, // ✅ при нажатии придёт callback_query.data
		}

		row = append(row, cell)

		// Если набрали 7 кнопок — добавляем строку
		if len(row)%AmountOfDaysInWeek == 0 {
			cal.addRowToKeyboard(&row)
			row = []telegram.InlineKeyboardButton{}
		}
	}

	// Пустые кнопки в конце (если строка неполная)
	if len(row) > 0 {
		for len(row) < AmountOfDaysInWeek {
			cal.addEmptyCell(&row)
		}
		cal.addRowToKeyboard(&row)
	}
}

// Builds a row of control buttons for swiping the calendar
func (cal *Calendar) addControlButtonsRow() {
	var row []telegram.InlineKeyboardButton

	prev := telegram.InlineKeyboardButton{
		Text:         "＜",
		CallbackData: "cal:nav:prev",
	}

	// Hide "prev" button if it rests on the range
	if cal.currYear <= cal.opt.YearRange[0] && cal.currMonth == 1 {
		prev.Text = " "
		prev.CallbackData = "cal:noop"
	}

	next := telegram.InlineKeyboardButton{
		Text:         "＞",
		CallbackData: "cal:nav:next",
	}

	// Hide "next" button if it rests on the range
	if cal.currYear >= cal.opt.YearRange[1] && cal.currMonth == 12 {
		next.Text = " "
		next.CallbackData = "cal:noop"
	}

	row = append(row, prev, next)
	cal.addRowToKeyboard(&row)
}

// Returns a formatted date string from the selected date
func (cal *Calendar) genDateStrFromDay(day int) string {
	return time.Date(cal.currYear, cal.currMonth, day,
		0, 0, 0, 0, time.UTC).Format("02.01.2006")
}

// Utility function for passing a row to the calendar's keyboard
func (cal *Calendar) addRowToKeyboard(row *[]telegram.InlineKeyboardButton) {
	cal.kb = append(cal.kb, *row)
}

// Inserts an empty button that doesn't do anything
func (cal *Calendar) addEmptyCell(row *[]telegram.InlineKeyboardButton) {
	*row = append(*row, telegram.InlineKeyboardButton{
		Text:         " ",
		CallbackData: "cal:noop",
	})
}
