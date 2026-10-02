package storage

import "sync"

// Хранилища создаются раньше журнала (logging.Service сам зависит от
// настроек), поэтому события восстановления после повреждённых файлов
// (карантин, откат к .bak) записываются в отложенный буфер и выгружаются
// в журнал при связывании приложения (cmd/awg-manager/wiring). До выгрузки
// каждое событие дублируется в stderr — если демон падает до подключения
// журнала, след остаётся хотя бы там.
type Notice struct {
	Action  string
	Target  string
	Message string
}

// maxPendingNotices ограничивает довайринговый буфер: при систематическом
// сбое (например, карантин не срабатывает на read-only ФС и Load повторяется
// каждый тик) буфер не должен расти безгранично.
const maxPendingNotices = 64

var (
	noticesMu      sync.Mutex
	pendingNotices []Notice
	noticeSink     func(Notice)
)

func recordNotice(action, target, message string) {
	n := Notice{Action: action, Target: target, Message: message}
	noticesMu.Lock()
	sink := noticeSink
	if sink == nil {
		if len(pendingNotices) < maxPendingNotices {
			pendingNotices = append(pendingNotices, n)
		}
		noticesMu.Unlock()
		return
	}
	noticesMu.Unlock()
	sink(n)
}

// SetNoticeSink подключает живой приёмник (журнал приложения) и выгружает
// в него всё накопленное до подключения. Большинство хранилищ грузятся
// лениво ПОСЛЕ создания logging.Service — без живого sink их события
// восстановления терялись бы навсегда.
func SetNoticeSink(sink func(Notice)) {
	noticesMu.Lock()
	noticeSink = sink
	buffered := pendingNotices
	pendingNotices = nil
	noticesMu.Unlock()
	for _, n := range buffered {
		sink(n)
	}
}
