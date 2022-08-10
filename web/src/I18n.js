const I18n = {
    lang: 'en',
    dict: {'en': {}},
    getlocale() { const locale =  navigator.language || navigator.browserLanguage || ( navigator.languages || [ "en" ]) [0]; 
                  return locale.startsWith('ru')? 'ru':'en'
                },
    getlang() { return this.lang },
    setlang(lang) { if (this.dict.hasOwnProperty(lang)) { this.lang = lang; } return this.lang },
    setdict(dict) { this.dict = dict },
    updatedict(dict) { this.dict = Object.assign(this.dict, dict) },
    make_dict_en(dict) { for (let k in dict) { this.dict.en[k] = k } },
    get(text)   { return this.dict[this.lang].hasOwnProperty(text)? this.dict[this.lang][text]: text },
    retranslate() {
            const strings = document.querySelectorAll(".translated");
            strings.forEach(function(item) {
                item.innerText = I18n.get(item.innerText)
            });
    }
 };

const dict = {
'ru': {
    "Read": "Прочитать",
    "Write": "Записать",
    'Time before reboot': 'Время до перезагрузки',
    'The duration of the "Reset" signal pulse': 'Длительность импульса сигнала "Reset"',
    'Hard reset sequence: hold the "Power" button for': 'Жесткая перезагрузка: зажать кнопку "Power" на',
    'Timeout between "Power" pulses': 'Задержка между импульсами сигналов "Power"',
    'The duration of the "Power" signal pulse (On)': 'Длительность импульса сигнала "Power" (вкл)',
    'PC will be restarted if there has been no signal from the app for': 'ПК будет перезагружен если от приложения не было сигнала в течение',
    'When restarting the PC, hold the "Reset" button for': 'При перезагрузке ПК держать кнопку "Reset"',
    'Release, wait': 'Отпустить, через',
    'Press the button for': 'Зажать кнопку на',
    'Channel 1': 'Канал 1',
    'Channel 2': "Канал 2",
    'Channel IN': 'Канал IN',
    'Reserved': 'Резерв',
    'min.': 'мин.',
    'sec.': 'сек.',
    'msec.': 'мсек.',

    'Off' : 'Выкл.',
    'Out opened': 'Вых. открыт',
    'Out closed': 'Вых. закрыт',
    'Temperature Threshold': 'Порог температуры',
    'Reset Limit': 'Ограничение перезагрузок',
    'Temp.sensor': 'Термодатчик',
    'Input': 'Вход',
    'Reset': 'Перезагрузка',
    'Power': 'Перезагрузка',
    'Shutdown': 'Выключение',
    'Main':'Главная',
    'Settings': 'Настройки',
    'Connect': 'Connect',
    'For this function your balance must be greater than 0' : 'Для работы этой функции ваш баланс должен быть больше 0',
    'Select process': 'Выбрать процесс',
    'Network monitoring': 'Монитор ip адреса',
    'Process monitoring': 'Монитор процесса',
    'Led': 'Светодиод',
    'Pause': 'Пауза',
    'Login to Connect!': 'Вход в Connect!',
    'Simple and easy to use cloud system': 'Простая облачная экосистема',
    'Login': 'Вход',
    "Don't have an account?" : 'Нет аккаунта?',
    'Register': 'Регистрация',
    'Account': 'Аккаунт',
    'Device': 'Имя устройства',
    'Alert': 'Оповещения',
    'Source': 'Источник',
    'Value': 'Значение',
    'Period': 'Период',
    'Balance': 'Баланс',
    'Load': 'Загрузка',
    'Monitor': 'Монитор',
    'Temperature': 'Температура',
    "d":"д",
    "h":"ч",
    "m":"м",
    "s":"с",
    "Settings updated": "Настройки обновлены",
    "Error": "Ошибка",
    "Settings read": "Настройки прочитаны",
    "Wrong parameters": "Ошибка в параметрах"
    }
};

I18n.updatedict(dict);
I18n.make_dict_en(dict.ru);
I18n.setlang(I18n.getlocale())

export default I18n;