--
-- PostgreSQL database dump
--

-- Dumped from database version 17.11 (Debian 17.11-0+deb13u1)
-- Dumped by pg_dump version 17.11 (Debian 17.11-0+deb13u1)

--
-- Name: channel; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.channel (
    id smallint NOT NULL,
    name character varying NOT NULL
);

--
-- Name: TABLE channel; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.channel IS '1=Скорозвон 2=вебхук-партнёр 3=Битрикс';

--
-- Name: dedup_provider; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dedup_provider (
    id integer NOT NULL,
    name character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL
);

--
-- Name: dedup_result; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dedup_result (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    provider_id integer NOT NULL,
    value_id smallint NOT NULL,
    checked_at timestamp without time zone NOT NULL,
    raw_response character varying
);

--
-- Name: TABLE dedup_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.dedup_result IS 'Актуальный результат = последняя запись по (lead_row_id, provider_id, checked_at DESC)';

--
-- Name: COLUMN dedup_result.raw_response; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dedup_result.raw_response IS 'сырой ответ провайдера. Nullable';

--
-- Name: dedup_value; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dedup_value (
    id smallint NOT NULL,
    provider_id integer NOT NULL,
    name character varying NOT NULL
);

--
-- Name: COLUMN dedup_value.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dedup_value.name IS 'Заря v1 (старый): Чистый/ДЮ/ДС<90/ДС>90/Ошибка. Заря v2 (новый): true/false — уточнить у Вовы';

--
-- Name: dispatch; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dispatch (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    channel_id smallint NOT NULL,
    upk_id integer,
    skorozvon_project_id character varying,
    sent_at timestamp without time zone NOT NULL,
    sent_ok boolean NOT NULL,
    external_id character varying,
    attempts smallint DEFAULT 1 NOT NULL,
    http_status smallint,
    raw_response text
);

--
-- Name: TABLE dispatch; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.dispatch IS 'UNIQUE (lead_row_id, channel_id, upk_id). Индексы: (upk_id, sent_at), (sent_at). Пишется только после подтверждения от внешней системы';

--
-- Name: COLUMN dispatch.upk_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dispatch.upk_id IS 'nullable — только для channel=вебхук-партнёр';

--
-- Name: COLUMN dispatch.skorozvon_project_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dispatch.skorozvon_project_id IS 'nullable — только для channel=Скорозвон';

--
-- Name: COLUMN dispatch.external_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dispatch.external_id IS 'id организации в СК или ответ вебхука партнёра. Nullable';

--
-- Name: COLUMN dispatch.http_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dispatch.http_status IS 'HTTP код ответа партнёра / СК. NULL если сеть упала до ответа';

--
-- Name: COLUMN dispatch.raw_response; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.dispatch.raw_response IS 'Сырое тело ответа. Nullable';

--
-- Name: e_dmp_tmp_leads_2; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.e_dmp_tmp_leads_2 (
    phone text,
    source_id text,
    region text,
    operator text,
    city text,
    name text,
    owner_id text,
    date_create timestamp without time zone,
    web_id text,
    tag text,
    call_id text,
    call_url text,
    call_last_at timestamp without time zone,
    project_num text,
    call_form text,
    dedup_last_result text,
    status_id text,
    date_modify timestamp without time zone,
    operator_id_raw text,
    city_alt text,
    cold_label text,
    base_label text,
    upk text,
    partner_sent_at text,
    title text,
    comments text,
    address text,
    legacy_fields text
);

--
-- Name: event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.event (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    event_type_id smallint NOT NULL,
    occurred_at timestamp without time zone NOT NULL,
    source smallint NOT NULL,
    details character varying
);

--
-- Name: TABLE event; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.event IS 'APPEND ONLY — только INSERT, никаких UPDATE/DELETE. Индексы: (lead_row_id, occurred_at), (event_type_id, occurred_at)';

--
-- Name: COLUMN event.source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event.source IS '1=Worker / 2=Скорозвон / 3=API / 4=внешнее';

--
-- Name: COLUMN event.details; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.event.details IS 'причина отклонения, комментарий. Nullable';

--
-- Name: event_type; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.event_type (
    id smallint NOT NULL,
    name character varying NOT NULL
);

--
-- Name: TABLE event_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.event_type IS '1=принята 2=обогащена 3=в СК 4=из СК 5=партнёру 6=в Б24 7=отклонена 8=архивирована';

--
-- Name: lead_row; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.lead_row (
    id bigint NOT NULL,
    uuid character varying NOT NULL,
    phone character varying NOT NULL,
    phone_valid boolean DEFAULT false NOT NULL,
    source_id integer,
    child_source_id integer,
    region_id integer,
    region_source smallint,
    city character varying,
    operator_id integer,
    operator_source smallint,
    web_id character varying,
    tag character varying,
    call_result bigint,
    call_id character varying,
    call_url character varying,
    call_last_at timestamp without time zone,
    project_num character varying,
    call_form character varying,
    status_id smallint NOT NULL,
    s3_key character varying,
    archived_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    legacy_fields text,
    bitrix_lead_id character varying,
    cold_label character varying,
    base_label character varying,
    source_description text,
    skorozvon_lead_id character varying,
    skorozvon_anketa_id bigint,
    via_us boolean DEFAULT true NOT NULL,
    call_result_name character varying,
    operator_raw character varying
);

--
-- Name: TABLE lead_row; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.lead_row IS 'Три писателя: API (приём + s3_key), Worker (обогащение), API-webhook (call_* + call_form от Скорозвона). Индексы: phone, web_id, source_id+created_at, status_id, archived_at';

--
-- Name: COLUMN lead_row.id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.id IS 'внутренний ключ';

--
-- Name: COLUMN lead_row.uuid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.uuid IS 'сквозной id наружу (Скорозвон external_id). Пишет API';

--
-- Name: COLUMN lead_row.phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.phone IS 'нормализованный E.164. Сырой в outbox, Worker нормализует';

--
-- Name: COLUMN lead_row.phone_valid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.phone_valid IS 'false = битый, в обзвон не идёт. Пишет Worker';

--
-- Name: COLUMN lead_row.source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.source_id IS 'родительский источник от токена. Пишет Worker';

--
-- Name: COLUMN lead_row.child_source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.child_source_id IS 'под-источник после дробления. Пишет Worker. Nullable';

--
-- Name: COLUMN lead_row.region_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.region_id IS 'результат каскада. Nullable';

--
-- Name: COLUMN lead_row.region_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.region_source IS '1=от поставщика / 2=Росреестр / 3=оператор / 0=не определён';

--
-- Name: COLUMN lead_row.city; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.city IS 'город клиента из анкеты оператора. Nullable';

--
-- Name: COLUMN lead_row.operator_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.operator_id IS 'снимок оператора на момент обработки. Nullable';

--
-- Name: COLUMN lead_row.operator_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.operator_source IS '1=от поставщика / 2=определён по номеру / 0=не определён';

--
-- Name: COLUMN lead_row.web_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.web_id IS 'id строки у поставщика. Ключ идемпотентности приёма. Nullable';

--
-- Name: COLUMN lead_row.tag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.tag IS 'тег строки. Nullable';

--
-- Name: COLUMN lead_row.call_result; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_result IS 'id результата сценария Скорозвона. Nullable';

--
-- Name: COLUMN lead_row.call_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_id IS 'id звонка в Скорозвоне. Nullable';

--
-- Name: COLUMN lead_row.call_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_url IS 'ссылка на запись разговора. Nullable';

--
-- Name: COLUMN lead_row.call_last_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_last_at IS 'время последнего звонка. Nullable';

--
-- Name: COLUMN lead_row.project_num; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.project_num IS 'id проекта Скорозвона после отправки; при приёме может быть номер проекта поставщика. Nullable';

--
-- Name: COLUMN lead_row.call_form; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_form IS 'jsonb: ответы анкеты оператора. Ключи: imya summa_dolga vid_dolga dohod zalog imuschestvo bankrotstvo gorod. Nullable';

--
-- Name: COLUMN lead_row.status_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.status_id IS '1=принята 2=обогащена 3=в СК 4=из СК 5=в Б24 6=отклонена 7=ожидание';

--
-- Name: COLUMN lead_row.s3_key; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.s3_key IS 'ключ объекта в S3. NULL у мигрированных исторических строк до выгрузки в S3';

--
-- Name: COLUMN lead_row.archived_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.archived_at IS 'дата архивации. NULL = активная строка. NOT NULL = в архиве S3';

--
-- Name: COLUMN lead_row.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.created_at IS 'время приёма. Пишет API';

--
-- Name: COLUMN lead_row.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.updated_at IS 'время последнего обновления. Nullable';

--
-- Name: COLUMN lead_row.legacy_fields; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.legacy_fields IS 'json-слепок легаси-полей Bitrix при миграции';

--
-- Name: COLUMN lead_row.bitrix_lead_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.bitrix_lead_id IS 'ID лида в Bitrix (owner_id из выгрузки)';

--
-- Name: COLUMN lead_row.cold_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.cold_label IS 'Холодная база после дробления. Пишет Worker из source_split';

--
-- Name: COLUMN lead_row.base_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.base_label IS 'Какая база после дробления. Пишет Worker из source_split';

--
-- Name: COLUMN lead_row.source_description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.source_description IS 'SOURCE_DESCRIPTION из Bitrix. Исторические строки — миграция по bitrix_lead_id';

--
-- Name: COLUMN lead_row.skorozvon_lead_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.skorozvon_lead_id IS 'id лида в Скорозвоне после успешной отправки. Ищем call_result по нему. Nullable';

--
-- Name: COLUMN lead_row.skorozvon_anketa_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.skorozvon_anketa_id IS 'актуальная анкета Скорозвона. История в skorozvon_anketa. Старый call_form не трогаем';

--
-- Name: COLUMN lead_row.via_us; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.via_us IS 'false = строка создана из webhook Скорозвона, лид не проходил наш приём. true = приняли мы';

--
-- Name: COLUMN lead_row.call_result_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.call_result_name IS 'название результата сценария Скорозвона (call_result.result_name). Nullable';

--
-- Name: COLUMN lead_row.operator_raw; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.lead_row.operator_raw IS 'Сырое значение оператора от поставщика или определителя до верхнего маппинга. Пусто — не было чего резолвить.';

--
-- Name: operator; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operator (
    id integer NOT NULL,
    name character varying NOT NULL,
    bitrix_name character varying
);

--
-- Name: COLUMN operator.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator.name IS 'БИЛАЙН / МТС / МЕГАФОН / ТЕЛЕ-2 / ДРУГИЕ';

--
-- Name: COLUMN operator.bitrix_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.operator.bitrix_name IS 'оригинальное название из Bitrix для маппинга при миграции. Nullable';

--
-- Name: operator_alias; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operator_alias (
    alias character varying NOT NULL,
    operator_id integer NOT NULL
);

--
-- Name: TABLE operator_alias; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.operator_alias IS 'Сырые названия оператора (поставщик / определитель) → внутренняя группа operator.id';

--
-- Name: outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    source_id integer NOT NULL,
    payload character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    locked_until timestamp without time zone,
    locked_by uuid
);

--
-- Name: TABLE outbox; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.outbox IS 'Worker читает напрямую через SELECT SKIP LOCKED — без Rabbit. Удаление после обработки. Самоочищается';

--
-- Name: COLUMN outbox.source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox.source_id IS 'источник строки — Worker читает отсюда без обращения к lead_row';

--
-- Name: COLUMN outbox.payload; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox.payload IS 'сырые данные для Worker: телефон, регион, оператор от поставщика';

--
-- Name: COLUMN outbox.locked_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox.locked_until IS 'пока > now() строка занята воркером; NULL или прошлое = можно брать';

--
-- Name: COLUMN outbox.locked_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox.locked_by IS 'токен текущего claim; DELETE/UNLOCK только с этим токеном';

--
-- Name: outbox_bx24; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox_bx24 (
    uuid character varying NOT NULL,
    uuid_lead character varying NOT NULL,
    locked_until timestamp with time zone
);

--
-- Name: TABLE outbox_bx24; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.outbox_bx24 IS 'очередь отправки в Б24. Пишем в той же TX, что status_id=4. Claim позже, как outbox';

--
-- Name: COLUMN outbox_bx24.uuid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox_bx24.uuid IS 'id задачи в очереди';

--
-- Name: COLUMN outbox_bx24.uuid_lead; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox_bx24.uuid_lead IS 'lead_row.uuid';

--
-- Name: COLUMN outbox_bx24.locked_until; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.outbox_bx24.locked_until IS 'пока > now() строка занята воркером; NULL или прошлое = можно брать';

--
-- Name: region; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.region (
    id integer NOT NULL,
    name character varying NOT NULL,
    code character varying
);

--
-- Name: COLUMN region.code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.region.code IS 'ОКТМО или внешний код. Nullable';

--
-- Name: region_alias; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.region_alias (
    alias text NOT NULL,
    region_id integer NOT NULL
);

--
-- Name: skorozvon_anketa; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_anketa (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    skorozvon_lead_id character varying NOT NULL,
    call_id character varying,
    user_id bigint,
    form_id bigint,
    scenario_id bigint,
    form_name character varying,
    answers character varying DEFAULT '[]'::character varying NOT NULL,
    form_updated_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);

--
-- Name: TABLE skorozvon_anketa; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_anketa IS 'анкета Скорозвона из webhook form_response. История: одна строка на событие. Актуальная — lead_row.skorozvon_anketa_id';

--
-- Name: COLUMN skorozvon_anketa.lead_row_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa.lead_row_id IS 'наша строка. Ищем по lead_row.skorozvon_lead_id = lead.id из webhook';

--
-- Name: COLUMN skorozvon_anketa.skorozvon_lead_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa.skorozvon_lead_id IS 'lead.id из Скорозвона';

--
-- Name: COLUMN skorozvon_anketa.call_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa.call_id IS 'call.id из webhook. Nullable';

--
-- Name: COLUMN skorozvon_anketa.answers; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa.answers IS 'json-массив answers: question_id, question_name, answer_id, answer_values';

--
-- Name: COLUMN skorozvon_anketa.form_updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa.form_updated_at IS 'form_response.updated_at из Скорозвона';

--
-- Name: skorozvon_anketa_fields; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_anketa_fields (
    id bigint NOT NULL,
    lead_row_id bigint NOT NULL,
    skorozvon_lead_id character varying NOT NULL,
    field_id bigint NOT NULL,
    enum_id bigint NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);

--
-- Name: TABLE skorozvon_anketa_fields; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_anketa_fields IS 'custom_fields карточки СК с call_result/form_response. Снимок: field_id + enum_id (option id). Подписи потом из skorozvon_maps';

--
-- Name: COLUMN skorozvon_anketa_fields.field_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa_fields.field_id IS 'id поля СК, в webhook ключ FIELD_{field_id}';

--
-- Name: COLUMN skorozvon_anketa_fields.enum_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_anketa_fields.enum_id IS 'id опции select/multiselect (skorozvon_maps.option_id)';

--
-- Name: skorozvon_anketa_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

--
-- Name: skorozvon_link; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_link (
    id integer NOT NULL,
    name character varying NOT NULL,
    type character varying NOT NULL,
    project_id bigint NOT NULL,
    created_at timestamp without time zone NOT NULL
);

--
-- Name: TABLE skorozvon_link; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_link IS 'Связка: под-источники + регионы → проект Скорозвона. type=dup|uniq. 1 под-источник = стандартная, 2+ = кастомная. Пустые регионы = все';

--
-- Name: COLUMN skorozvon_link.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_link.type IS 'dup = работа с дублями, uniq = работа с униками';

--
-- Name: COLUMN skorozvon_link.project_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_link.project_id IS 'call_project_id из skorozvon_project';

--
-- Name: skorozvon_link_region; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_link_region (
    link_id integer NOT NULL,
    region_id integer NOT NULL
);

--
-- Name: TABLE skorozvon_link_region; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_link_region IS 'Разрешённые регионы связки. Пусто = регион не проверяется';

--
-- Name: skorozvon_link_source; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_link_source (
    link_id integer NOT NULL,
    source_id integer NOT NULL
);

--
-- Name: TABLE skorozvon_link_source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_link_source IS 'Под-источники связки';

--
-- Name: skorozvon_maps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_maps (
    field_id bigint NOT NULL,
    option_id bigint DEFAULT 0 NOT NULL,
    name text NOT NULL,
    field_type character varying,
    lead_type character varying,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    bitrix_code character varying
);

--
-- Name: TABLE skorozvon_maps; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_maps IS 'справочник настраиваемых полей Скорозвона (GET /api/v2/custom_fields). Пишет worker в 12:00 и 19:00 Europe/Moscow';

--
-- Name: COLUMN skorozvon_maps.field_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.field_id IS 'id поля в СК. В webhook ключ FIELD_{field_id}';

--
-- Name: COLUMN skorozvon_maps.option_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.option_id IS '0 = само поле (name = название поля). Иначе id опции select/multiselect (name = label опции)';

--
-- Name: COLUMN skorozvon_maps.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.name IS 'название поля или подпись опции';

--
-- Name: COLUMN skorozvon_maps.field_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.field_type IS 'date, multiselect, select, link, text, textarea, number, comment';

--
-- Name: COLUMN skorozvon_maps.lead_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.lead_type IS 'lead или organization';

--
-- Name: COLUMN skorozvon_maps.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.updated_at IS 'когда worker последний раз обновил строку';

--
-- Name: COLUMN skorozvon_maps.bitrix_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_maps.bitrix_code IS 'Код поля Битрикса для этого названия из Скорозвона. Пусто — поле в Б24 не мапится.';

--
-- Name: skorozvon_project; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skorozvon_project (
    uuid character varying NOT NULL,
    project_id bigint NOT NULL,
    name text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    last_date_request timestamp without time zone,
    last_request boolean
);

--
-- Name: TABLE skorozvon_project; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.skorozvon_project IS 'Реестр проектов Скорозвона. project_id = call_project_id';

--
-- Name: COLUMN skorozvon_project.uuid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_project.uuid IS 'внутренний ключ';

--
-- Name: COLUMN skorozvon_project.project_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_project.project_id IS 'id проекта в Скорозвоне (call_project_id)';

--
-- Name: COLUMN skorozvon_project.last_date_request; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_project.last_date_request IS 'когда последний раз проверяли, что проект ещё есть';

--
-- Name: COLUMN skorozvon_project.last_request; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.skorozvon_project.last_request IS 'true = Скорозвон ответил 200';

--
-- Name: source; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.source (
    id integer NOT NULL,
    parent_id integer,
    name character varying NOT NULL,
    bitrix_code character varying,
    token character varying,
    cold_label character varying,
    base_label character varying,
    is_active boolean DEFAULT true NOT NULL,
    idempotency_active boolean DEFAULT false NOT NULL,
    teg_active boolean DEFAULT false NOT NULL,
    teg_expected_value character varying
);

--
-- Name: TABLE source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.source IS 'Self-reference: parent_id ссылается на id этой же таблицы. При миграции: сначала корневые (parent_id=NULL), потом под-источники';

--
-- Name: COLUMN source.parent_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.parent_id IS 'NULL = корневой поставщик. Заполнен = под-источник (наш внутренний id, не из Bitrix)';

--
-- Name: COLUMN source.bitrix_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.bitrix_code IS 'код UC_* или числовой id из Bitrix. Для маппинга при миграции. Nullable';

--
-- Name: COLUMN source.token; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.token IS 'токен хука для приёма. Только у корневых. Хранится в открытом виде. Nullable';

--
-- Name: COLUMN source.cold_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.cold_label IS 'метка холодной базы. Nullable';

--
-- Name: COLUMN source.base_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.base_label IS 'метка какой базы. Nullable';

--
-- Name: COLUMN source.is_active; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.is_active IS 'false = приём остановлен (403 Forbidden)';

--
-- Name: COLUMN source.idempotency_active; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.idempotency_active IS 'true = идемпотентность приёма по web_id в Redis, TTL 24 часа';

--
-- Name: COLUMN source.teg_active; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.teg_active IS 'Проверка teg после дробления по оператору. Только у корня';

--
-- Name: COLUMN source.teg_expected_value; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source.teg_expected_value IS 'Ожидаемое значение teg (например подтверждён). Не совпало — тестовый под-источник правила';

--
-- Name: source_skorozvon; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.source_skorozvon (
    id integer NOT NULL,
    source_id integer NOT NULL,
    project_id character varying NOT NULL
);

--
-- Name: COLUMN source_skorozvon.project_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_skorozvon.project_id IS 'id проекта в Скорозвоне (call_project_id)';

--
-- Name: source_split; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.source_split (
    id integer NOT NULL,
    source_id integer NOT NULL,
    operator_id integer NOT NULL,
    child_source_id integer NOT NULL,
    test_child_source_id integer,
    cold_label character varying,
    base_label character varying
);

--
-- Name: TABLE source_split; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.source_split IS 'UNIQUE (source_id, operator_id). Одно правило на пару (источник + оператор). Несколько операторов = несколько строк';

--
-- Name: COLUMN source_split.source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_split.source_id IS 'родительский источник';

--
-- Name: COLUMN source_split.child_source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_split.child_source_id IS 'результирующий под-источник';

--
-- Name: COLUMN source_split.test_child_source_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_split.test_child_source_id IS 'Куда уходит строка, если teg не совпал. Nullable если проверка выключена';

--
-- Name: COLUMN source_split.cold_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_split.cold_label IS 'Холодная база этого правила. Не общая на под-источник';

--
-- Name: COLUMN source_split.base_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_split.base_label IS 'Какая база этого правила. Не общая на под-источник';

--
-- Name: source_upk; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.source_upk (
    source_id integer NOT NULL,
    upk_id integer NOT NULL,
    allowed boolean NOT NULL
);

--
-- Name: COLUMN source_upk.allowed; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.source_upk.allowed IS 'true = можно продавать / false = стоп-лист';

--
-- Name: status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.status (
    id smallint NOT NULL,
    name character varying NOT NULL
);

--
-- Name: TABLE status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.status IS '1=принята 2=обогащена 3=в СК 4=из СК 5=в Б24 6=отклонена 7=ожидание';

--
-- Name: upk; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upk (
    id integer NOT NULL,
    name character varying NOT NULL,
    webhook_url character varying NOT NULL,
    dedup_days smallint,
    is_active boolean DEFAULT true NOT NULL,
    zarya_results text[] DEFAULT '{}'::text[] NOT NULL,
    last_date_request timestamp without time zone,
    last_request boolean,
    last_http_status integer,
    last_response text,
    body jsonb DEFAULT '{}'::jsonb NOT NULL,
    content_type character varying DEFAULT 'application/x-www-form-urlencoded'::character varying NOT NULL
);

--
-- Name: COLUMN upk.name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.name IS 'РТ / КР / ББ / ...';

--
-- Name: COLUMN upk.dedup_days; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.dedup_days IS 'период дедупа по номеру в днях. NULL = без ограничений';

--
-- Name: COLUMN upk.zarya_results; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.zarya_results IS 'Что принимает от Зари: dup, unique, error';

--
-- Name: COLUMN upk.last_date_request; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.last_date_request IS 'время последней проверки webhook';

--
-- Name: COLUMN upk.last_request; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.last_request IS 'последняя проверка: HTTP 2xx';

--
-- Name: COLUMN upk.last_http_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.last_http_status IS 'HTTP-статус последней проверки';

--
-- Name: COLUMN upk.last_response; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.last_response IS 'тело ответа последней проверки';

--
-- Name: COLUMN upk.body; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.body IS 'Параметры запроса: уходят и в query (GET), и в тело POST. Плейсхолдеры {{phone}}, {{phone_valid}}, {{region}}, {{city}}';

--
-- Name: COLUMN upk.content_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN public.upk.content_type IS 'Content-Type POST-тела';

--
-- Name: upk_region; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upk_region (
    upk_id integer NOT NULL,
    region_id integer NOT NULL
);

--
-- Name: TABLE upk_region; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE public.upk_region IS 'Разрешённые регионы покупателя. Пусто = регион не проверяется';

--
-- Name: channel channel_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.channel
    ADD CONSTRAINT channel_pkey PRIMARY KEY (id);

--
-- Name: dedup_provider dedup_provider_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_provider
    ADD CONSTRAINT dedup_provider_pkey PRIMARY KEY (id);

--
-- Name: dedup_result dedup_result_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_pkey PRIMARY KEY (id);

--
-- Name: dedup_value dedup_value_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_value
    ADD CONSTRAINT dedup_value_pkey PRIMARY KEY (id);

--
-- Name: dispatch dispatch_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_pkey PRIMARY KEY (id);

--
-- Name: event event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event
    ADD CONSTRAINT event_pkey PRIMARY KEY (id);

--
-- Name: event_type event_type_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event_type
    ADD CONSTRAINT event_type_pkey PRIMARY KEY (id);

--
-- Name: lead_row lead_row_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_pkey PRIMARY KEY (id);

--
-- Name: lead_row lead_row_uuid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_uuid_key UNIQUE (uuid);

--
-- Name: operator_alias operator_alias_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.operator_alias
    ADD CONSTRAINT operator_alias_pkey PRIMARY KEY (alias);

--
-- Name: operator operator_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.operator
    ADD CONSTRAINT operator_pkey PRIMARY KEY (id);

--
-- Name: outbox_bx24 outbox_bx24_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox_bx24
    ADD CONSTRAINT outbox_bx24_pkey PRIMARY KEY (uuid);

--
-- Name: outbox outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox
    ADD CONSTRAINT outbox_pkey PRIMARY KEY (id);

--
-- Name: region_alias region_alias_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.region_alias
    ADD CONSTRAINT region_alias_pkey PRIMARY KEY (alias);

--
-- Name: region region_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.region
    ADD CONSTRAINT region_pkey PRIMARY KEY (id);

--
-- Name: skorozvon_anketa_fields skorozvon_anketa_fields_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_anketa_fields
    ADD CONSTRAINT skorozvon_anketa_fields_pkey PRIMARY KEY (id);

--
-- Name: skorozvon_anketa skorozvon_anketa_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_anketa
    ADD CONSTRAINT skorozvon_anketa_pkey PRIMARY KEY (id);

--
-- Name: skorozvon_link skorozvon_link_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link
    ADD CONSTRAINT skorozvon_link_pkey PRIMARY KEY (id);

--
-- Name: skorozvon_link_region skorozvon_link_region_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_region
    ADD CONSTRAINT skorozvon_link_region_pkey PRIMARY KEY (link_id, region_id);

--
-- Name: skorozvon_link_source skorozvon_link_source_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_source
    ADD CONSTRAINT skorozvon_link_source_pkey PRIMARY KEY (link_id, source_id);

--
-- Name: skorozvon_maps skorozvon_maps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_maps
    ADD CONSTRAINT skorozvon_maps_pkey PRIMARY KEY (field_id, option_id);

--
-- Name: skorozvon_project skorozvon_project_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_project
    ADD CONSTRAINT skorozvon_project_pkey PRIMARY KEY (uuid);

--
-- Name: skorozvon_project skorozvon_project_project_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_project
    ADD CONSTRAINT skorozvon_project_project_id_key UNIQUE (project_id);

--
-- Name: source source_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source
    ADD CONSTRAINT source_pkey PRIMARY KEY (id);

--
-- Name: source_skorozvon source_skorozvon_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_skorozvon
    ADD CONSTRAINT source_skorozvon_pkey PRIMARY KEY (id);

--
-- Name: source_split source_split_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_pkey PRIMARY KEY (id);

--
-- Name: source_upk source_upk_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_upk
    ADD CONSTRAINT source_upk_pkey PRIMARY KEY (source_id, upk_id);

--
-- Name: status status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.status
    ADD CONSTRAINT status_pkey PRIMARY KEY (id);

--
-- Name: upk upk_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.upk
    ADD CONSTRAINT upk_pkey PRIMARY KEY (id);

--
-- Name: upk_region upk_region_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.upk_region
    ADD CONSTRAINT upk_region_pkey PRIMARY KEY (upk_id, region_id);

--
-- Name: dedup_result_lead_provider_checked_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dedup_result_lead_provider_checked_idx ON public.dedup_result (lead_row_id, provider_id, checked_at DESC);

--
-- Name: dispatch_lead_channel_sent_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dispatch_lead_channel_sent_idx ON public.dispatch (lead_row_id, channel_id, sent_at DESC);

--
-- Name: dispatch_lead_channel_skorozvon_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dispatch_lead_channel_skorozvon_uidx ON public.dispatch (lead_row_id, channel_id, skorozvon_project_id) WHERE (skorozvon_project_id IS NOT NULL);

--
-- Name: dispatch_lead_channel_upk_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dispatch_lead_channel_upk_uidx ON public.dispatch (lead_row_id, channel_id, upk_id) WHERE (upk_id IS NOT NULL);

--
-- Name: dispatch_skorozvon_external_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dispatch_skorozvon_external_id_idx ON public.dispatch (external_id) WHERE ((channel_id = 1) AND (external_id IS NOT NULL));

--
-- Name: e_dmp_tmp_leads_2_date_create_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX e_dmp_tmp_leads_2_date_create_idx ON public.e_dmp_tmp_leads_2 (date_create);

--
-- Name: e_dmp_tmp_leads_2_owner_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX e_dmp_tmp_leads_2_owner_id_idx ON public.e_dmp_tmp_leads_2 (owner_id);

--
-- Name: e_dmp_tmp_leads_2_source_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX e_dmp_tmp_leads_2_source_id_idx ON public.e_dmp_tmp_leads_2 (source_id);

--
-- Name: lead_row_bitrix_lead_id_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX lead_row_bitrix_lead_id_uidx ON public.lead_row (bitrix_lead_id) WHERE (bitrix_lead_id IS NOT NULL);

--
-- Name: lead_row_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_created_at_idx ON public.lead_row (created_at);

--
-- Name: lead_row_operator_raw_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_operator_raw_idx ON public.lead_row (operator_id, operator_raw) WHERE ((operator_raw IS NOT NULL) AND ((operator_raw)::text <> ''::text));

--
-- Name: lead_row_phone_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_phone_idx ON public.lead_row (phone);

--
-- Name: lead_row_skorozvon_lead_id_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX lead_row_skorozvon_lead_id_uidx ON public.lead_row (skorozvon_lead_id) WHERE (skorozvon_lead_id IS NOT NULL);

--
-- Name: lead_row_source_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_source_created_idx ON public.lead_row (source_id, created_at);

--
-- Name: lead_row_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_status_idx ON public.lead_row (status_id);

--
-- Name: lead_row_via_us_false_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX lead_row_via_us_false_idx ON public.lead_row (id DESC) WHERE (NOT via_us);

--
-- Name: outbox_bx24_uuid_lead_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX outbox_bx24_uuid_lead_uidx ON public.outbox_bx24 (uuid_lead);

--
-- Name: outbox_locked_until_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX outbox_locked_until_idx ON public.outbox (locked_until) WHERE (locked_until IS NOT NULL);

--
-- Name: outbox_unlocked_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX outbox_unlocked_idx ON public.outbox (id) WHERE (locked_until IS NULL);

--
-- Name: skorozvon_anketa_fields_lead_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX skorozvon_anketa_fields_lead_idx ON public.skorozvon_anketa_fields (lead_row_id);

--
-- Name: skorozvon_anketa_fields_sk_field_enum_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX skorozvon_anketa_fields_sk_field_enum_uidx ON public.skorozvon_anketa_fields (skorozvon_lead_id, field_id, enum_id);

--
-- Name: skorozvon_anketa_fields_sk_lead_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX skorozvon_anketa_fields_sk_lead_idx ON public.skorozvon_anketa_fields (skorozvon_lead_id);

--
-- Name: skorozvon_anketa_lead_row_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX skorozvon_anketa_lead_row_id_idx ON public.skorozvon_anketa (lead_row_id, id DESC);

--
-- Name: source_split_source_operator_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX source_split_source_operator_uidx ON public.source_split (source_id, operator_id);

--
-- Name: source_token_uidx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX source_token_uidx ON public.source (token) WHERE ((token IS NOT NULL) AND ((token)::text <> ''::text));

--
-- Name: dedup_result dedup_result_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: dedup_result dedup_result_lead_row_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_lead_row_id_fkey1 FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: dedup_result dedup_result_provider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES public.dedup_provider(id);

--
-- Name: dedup_result dedup_result_provider_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_provider_id_fkey1 FOREIGN KEY (provider_id) REFERENCES public.dedup_provider(id);

--
-- Name: dedup_result dedup_result_value_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_value_id_fkey FOREIGN KEY (value_id) REFERENCES public.dedup_value(id);

--
-- Name: dedup_result dedup_result_value_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_result
    ADD CONSTRAINT dedup_result_value_id_fkey1 FOREIGN KEY (value_id) REFERENCES public.dedup_value(id);

--
-- Name: dedup_value dedup_value_provider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_value
    ADD CONSTRAINT dedup_value_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES public.dedup_provider(id);

--
-- Name: dedup_value dedup_value_provider_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dedup_value
    ADD CONSTRAINT dedup_value_provider_id_fkey1 FOREIGN KEY (provider_id) REFERENCES public.dedup_provider(id);

--
-- Name: dispatch dispatch_channel_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES public.channel(id);

--
-- Name: dispatch dispatch_channel_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_channel_id_fkey1 FOREIGN KEY (channel_id) REFERENCES public.channel(id);

--
-- Name: dispatch dispatch_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: dispatch dispatch_lead_row_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_lead_row_id_fkey1 FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: dispatch dispatch_upk_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_upk_id_fkey FOREIGN KEY (upk_id) REFERENCES public.upk(id);

--
-- Name: dispatch dispatch_upk_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.dispatch
    ADD CONSTRAINT dispatch_upk_id_fkey1 FOREIGN KEY (upk_id) REFERENCES public.upk(id);

--
-- Name: event event_event_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event
    ADD CONSTRAINT event_event_type_id_fkey FOREIGN KEY (event_type_id) REFERENCES public.event_type(id);

--
-- Name: event event_event_type_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event
    ADD CONSTRAINT event_event_type_id_fkey1 FOREIGN KEY (event_type_id) REFERENCES public.event_type(id);

--
-- Name: event event_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event
    ADD CONSTRAINT event_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: event event_lead_row_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.event
    ADD CONSTRAINT event_lead_row_id_fkey1 FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: lead_row lead_row_child_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_child_source_id_fkey FOREIGN KEY (child_source_id) REFERENCES public.source(id);

--
-- Name: lead_row lead_row_child_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_child_source_id_fkey1 FOREIGN KEY (child_source_id) REFERENCES public.source(id);

--
-- Name: lead_row lead_row_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.operator(id);

--
-- Name: lead_row lead_row_operator_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_operator_id_fkey1 FOREIGN KEY (operator_id) REFERENCES public.operator(id);

--
-- Name: lead_row lead_row_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.region(id);

--
-- Name: lead_row lead_row_region_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_region_id_fkey1 FOREIGN KEY (region_id) REFERENCES public.region(id);

--
-- Name: lead_row lead_row_skorozvon_anketa_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_skorozvon_anketa_id_fkey FOREIGN KEY (skorozvon_anketa_id) REFERENCES public.skorozvon_anketa(id);

--
-- Name: lead_row lead_row_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: lead_row lead_row_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_source_id_fkey1 FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: lead_row lead_row_status_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_status_id_fkey FOREIGN KEY (status_id) REFERENCES public.status(id);

--
-- Name: lead_row lead_row_status_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.lead_row
    ADD CONSTRAINT lead_row_status_id_fkey1 FOREIGN KEY (status_id) REFERENCES public.status(id);

--
-- Name: operator_alias operator_alias_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.operator_alias
    ADD CONSTRAINT operator_alias_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.operator(id);

--
-- Name: outbox_bx24 outbox_bx24_uuid_lead_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox_bx24
    ADD CONSTRAINT outbox_bx24_uuid_lead_fkey FOREIGN KEY (uuid_lead) REFERENCES public.lead_row(uuid);

--
-- Name: outbox outbox_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox
    ADD CONSTRAINT outbox_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: outbox outbox_lead_row_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox
    ADD CONSTRAINT outbox_lead_row_id_fkey1 FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: outbox outbox_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox
    ADD CONSTRAINT outbox_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: outbox outbox_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.outbox
    ADD CONSTRAINT outbox_source_id_fkey1 FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: region_alias region_alias_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.region_alias
    ADD CONSTRAINT region_alias_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.region(id);

--
-- Name: skorozvon_anketa_fields skorozvon_anketa_fields_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_anketa_fields
    ADD CONSTRAINT skorozvon_anketa_fields_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: skorozvon_anketa skorozvon_anketa_lead_row_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_anketa
    ADD CONSTRAINT skorozvon_anketa_lead_row_id_fkey FOREIGN KEY (lead_row_id) REFERENCES public.lead_row(id);

--
-- Name: skorozvon_link skorozvon_link_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link
    ADD CONSTRAINT skorozvon_link_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.skorozvon_project(project_id);

--
-- Name: skorozvon_link_region skorozvon_link_region_link_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_region
    ADD CONSTRAINT skorozvon_link_region_link_id_fkey FOREIGN KEY (link_id) REFERENCES public.skorozvon_link(id) ON DELETE CASCADE;

--
-- Name: skorozvon_link_region skorozvon_link_region_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_region
    ADD CONSTRAINT skorozvon_link_region_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.region(id);

--
-- Name: skorozvon_link_source skorozvon_link_source_link_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_source
    ADD CONSTRAINT skorozvon_link_source_link_id_fkey FOREIGN KEY (link_id) REFERENCES public.skorozvon_link(id) ON DELETE CASCADE;

--
-- Name: skorozvon_link_source skorozvon_link_source_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.skorozvon_link_source
    ADD CONSTRAINT skorozvon_link_source_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source source_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source
    ADD CONSTRAINT source_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.source(id);

--
-- Name: source source_parent_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source
    ADD CONSTRAINT source_parent_id_fkey1 FOREIGN KEY (parent_id) REFERENCES public.source(id);

--
-- Name: source_skorozvon source_skorozvon_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_skorozvon
    ADD CONSTRAINT source_skorozvon_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_skorozvon source_skorozvon_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_skorozvon
    ADD CONSTRAINT source_skorozvon_source_id_fkey1 FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_split source_split_child_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_child_source_id_fkey FOREIGN KEY (child_source_id) REFERENCES public.source(id);

--
-- Name: source_split source_split_child_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_child_source_id_fkey1 FOREIGN KEY (child_source_id) REFERENCES public.source(id);

--
-- Name: source_split source_split_operator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES public.operator(id);

--
-- Name: source_split source_split_operator_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_operator_id_fkey1 FOREIGN KEY (operator_id) REFERENCES public.operator(id);

--
-- Name: source_split source_split_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_split source_split_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_source_id_fkey1 FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_split source_split_test_child_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_split
    ADD CONSTRAINT source_split_test_child_source_id_fkey FOREIGN KEY (test_child_source_id) REFERENCES public.source(id);

--
-- Name: source_upk source_upk_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_upk
    ADD CONSTRAINT source_upk_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_upk source_upk_source_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_upk
    ADD CONSTRAINT source_upk_source_id_fkey1 FOREIGN KEY (source_id) REFERENCES public.source(id);

--
-- Name: source_upk source_upk_upk_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_upk
    ADD CONSTRAINT source_upk_upk_id_fkey FOREIGN KEY (upk_id) REFERENCES public.upk(id);

--
-- Name: source_upk source_upk_upk_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.source_upk
    ADD CONSTRAINT source_upk_upk_id_fkey1 FOREIGN KEY (upk_id) REFERENCES public.upk(id);

--
-- Name: upk_region upk_region_region_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.upk_region
    ADD CONSTRAINT upk_region_region_id_fkey FOREIGN KEY (region_id) REFERENCES public.region(id);

--
-- Name: upk_region upk_region_upk_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE public.upk_region
    ADD CONSTRAINT upk_region_upk_id_fkey FOREIGN KEY (upk_id) REFERENCES public.upk(id) ON DELETE CASCADE;

--
-- PostgreSQL database dump complete
--
