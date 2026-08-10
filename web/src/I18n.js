const I18n = {
  lang: 'en',
  dict: { en: {} },
  getlocale() {
    const locale = navigator.language || navigator.languages?.[0] || 'en';
    return locale.startsWith('ru') ? 'ru' : 'en';
  },
  setlang(lang) {
    if (Object.hasOwn(this.dict, lang)) this.lang = lang;
    if (typeof document !== 'undefined') document.documentElement.lang = this.lang;
    return this.lang;
  },
  updatedict(dict) {
    Object.assign(this.dict, dict);
  },
  make_dict_en(dict) {
    for (const key of Object.keys(dict)) this.dict.en[key] = key;
  },
  get(text) {
    return Object.hasOwn(this.dict[this.lang], text) ? this.dict[this.lang][text] : text;
  },
};

const dict = {
  ru: {
    'Read': 'Прочитать',
    'Write': 'Записать',
    'PC will be restarted if there has been no signal from the app for': 'ПК будет перезагружен если от приложения не было сигнала в течение',
    'When restarting the PC, hold the "Reset" button for': 'При перезагрузке ПК держать кнопку "Reset"',
    'When hard-restarting the PC, hold the "Power" button for': 'При жесткой перезагрузке ПК держать кнопку "Power"',
    'When hard-restarting the PC, after powering off, wait for': 'При жесткой перезагрузке ПК после выключения ждать',
    'When hard-restarting the PC, after powering off, hold the "Power" button for': 'При жесткой перезагрузке ПК после выключения держать кнопку "Power"',
    'Channel 1': 'Канал 1',
    'Channel 2': 'Канал 2',
    'Channel IN': 'Канал IN',
    'Reserved': 'Резерв',
    'min.': 'мин.',
    'sec.': 'сек.',
    'msec.': 'мсек.',

    'Off': 'Выкл.',
    'Out opened': 'Вых. открыт',
    'Out closed': 'Вых. закрыт',
    'Temperature Threshold': 'Порог температуры',
    'Reset Limit': 'Ограничение перезагрузок',
    'Temp.sensor': 'Термодатчик',
    'Input': 'Вход',
    'Reset': 'Перезагрузка',
    'Power': 'Питание',
    'Shutdown': 'Выключение',
    'Main': 'Главная',
    'Settings': 'Настройки',
    'Cloud': 'Облако',
    'Skip to main content': 'Перейти к основному содержимому',
    'Application controls': 'Управление приложением',
    'Device connected': 'Устройство подключено',
    'Device disconnected': 'Устройство не найдено',
    'Device status': 'Состояние устройства',
    'Monitoring settings': 'Настройки мониторинга',
    'Open process list': 'Открыть список процессов',
    'Close': 'Закрыть',
    'Close dialog backdrop': 'Закрыть диалог по фону',
    'Clear tokens': 'Очистить токены',
    'Show tokens': 'Показать токены',
    'Hide tokens': 'Скрыть токены',
    'Clear tokens confirmation': 'Удалить сохранённые облачные токены? Read-токен нельзя будет восстановить.',
    'Clearing…': 'Удаление…',
    'Failed to clear tokens': 'Не удалось удалить токены',
    'Loading…': 'Загрузка…',
    'No processes found': 'Процессы не найдены',
    'Failed to load processes': 'Не удалось загрузить процессы',
    'Select process': 'Выбрать процесс',
    'TCP endpoint monitoring': 'Мониторинг TCP-узла',
    'Host, host:port or URL. A plain host uses port 80.': 'Хост, host:port или URL. Для хоста без порта используется порт 80.',
    'Process monitoring': 'Монитор процесса',
    'Led': 'Светодиод',
    'Pause': 'Пауза',
    'Cloud Lite': 'Cloud Lite',
    'Cloud connection': 'Подключение к облаку',
    'Simple and easy to use cloud system': 'Простая облачная экосистема',
    'I already have a token': 'У меня уже есть токен',
    'Use existing token': 'Использовать существующий токен',
    'Get new tokens': 'Получить новые токены',
    'Validating…': 'Проверка…',
    'Creating…': 'Создание…',
    'Only the write token is saved. Keep your read token separately.': 'Сохраняется только write-токен. Read-токен храните отдельно.',
    'New read and write tokens will be shown once. Save them immediately.': 'Новые read/write токены будут показаны один раз. Сразу сохраните их.',
    'or': 'или',
    'Dashboard': 'Панель',
    'Write token': 'Write-токен',
    'Write token is configured.': 'Write-токен настроен.',
    'Read token': 'Read-токен',
    'Save these tokens. The read token is shown once and is not saved in settings.json.': 'Сохраните эти токены. Read-токен показывается один раз и не сохраняется в settings.json.',
    'Save this write token.': 'Сохраните этот write-токен.',
    'Device ID': 'ID устройства',
    'Monitor': 'Монитор',
    'd': 'д',
    'h': 'ч',
    'm': 'м',
    's': 'с',
    'Settings updated': 'Настройки обновлены',
    'Error': 'Ошибка',
    'Settings read': 'Настройки прочитаны',
    'Settings read failed': 'Не удалось прочитать настройки',
    'Reading…': 'Чтение…',
    'Writing…': 'Запись…',
    'Wrong parameters': 'Ошибка в параметрах',
    'Unexpected device response': 'Неожиданный ответ устройства',
    'Device command sent': 'Команда отправлена устройству',
    'Device command failed': 'Не удалось выполнить команду устройства',
    'Restart the connected PC now?': 'Перезагрузить подключённый ПК сейчас?',
    'Send the Power command to the connected PC now?': 'Отправить подключённому ПК команду Power сейчас?',
    'Shut down the connected PC now?': 'Выключить подключённый ПК сейчас?',
    'Failed to load settings': 'Не удалось загрузить настройки',
    'Failed to save settings': 'Не удалось сохранить настройки',
    'Invalid write token': 'Некорректный write-токен',
    'Cloud service unavailable': 'Облачный сервис недоступен',
    'Invalid cloud response': 'Некорректный ответ облачного сервиса',
  },
};

I18n.updatedict(dict);
I18n.make_dict_en(dict.ru);
I18n.setlang(I18n.getlocale());

export default I18n;
