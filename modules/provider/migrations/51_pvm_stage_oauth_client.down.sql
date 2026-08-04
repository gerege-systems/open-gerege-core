-- `public.applications` нь устгагдсан байж болно — up-тай ижил болзол.
DO $$
BEGIN
    IF to_regclass('public.applications') IS NOT NULL THEN
        DELETE FROM public.applications WHERE client_id = 'pvm-stagegerege-mn';
    END IF;
END $$;
DELETE FROM public.oauth_clients WHERE client_id = 'pvm-stagegerege-mn';
