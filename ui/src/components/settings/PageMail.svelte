<script>
    import tooltip from "@/actions/tooltip";
    import Field from "@/components/base/Field.svelte";
    import ObjectSelect from "@/components/base/ObjectSelect.svelte";
    import PageWrapper from "@/components/base/PageWrapper.svelte";
    import RedactedPasswordInput from "@/components/base/RedactedPasswordInput.svelte";
    import EmailTestPopup from "@/components/settings/EmailTestPopup.svelte";
    import SettingsSidebar from "@/components/settings/SettingsSidebar.svelte";
    import { pageTitle } from "@/stores/app";
    import { setErrors } from "@/stores/errors";
    import { addSuccessToast } from "@/stores/toasts";
    import ApiClient from "@/utils/ApiClient";
    import CommonHelper from "@/utils/CommonHelper";
    import { slide } from "svelte/transition";

    const emailProviderOptions = [
        { label: "None (local sendmail)", value: "" },
        { label: "SMTP", value: "smtp" },
        { label: "Resend", value: "resend" },
    ];

    const tlsOptions = [
        { label: "Auto (StartTLS)", value: false },
        { label: "Always", value: true },
    ];

    const authMethods = [
        { label: "PLAIN (default)", value: "PLAIN" },
        { label: "LOGIN", value: "LOGIN" },
    ];

    $pageTitle = "Mail settings";

    let testPopup;
    let originalFormSettings = {};
    let formSettings = {};
    let isLoading = false;
    let isSaving = false;
    let maskSmtpPassword = false;
    let maskResendApiKey = false;
    let showMoreOptions = false;

    $: initialHash = JSON.stringify(originalFormSettings);

    $: hasChanges = initialHash != JSON.stringify(formSettings);

    loadSettings();

    async function loadSettings() {
        isLoading = true;

        try {
            const settings = (await ApiClient.settings.getAll()) || {};
            init(settings);
        } catch (err) {
            ApiClient.error(err);
        }

        isLoading = false;
    }

    async function save() {
        if (isSaving || !hasChanges) {
            return;
        }

        isSaving = true;

        try {
            // Sync the enabled flags based on emailProvider selection
            const settingsToSave = CommonHelper.filterRedactedProps(formSettings);
            settingsToSave.smtp.enabled = settingsToSave.emailProvider === "smtp";
            settingsToSave.resend.enabled = settingsToSave.emailProvider === "resend";

            const settings = await ApiClient.settings.update(settingsToSave);
            init(settings);
            setErrors({});
            addSuccessToast("Successfully saved mail settings.");
        } catch (err) {
            ApiClient.error(err);
        }

        isSaving = false;
    }

    function init(settings = {}) {
        formSettings = {
            meta: settings?.meta || {},
            smtp: settings?.smtp || {},
            resend: settings?.resend || {},
            emailProvider: settings?.emailProvider || "",
        };

        // Backward compatibility: if smtp.enabled is true but emailProvider is empty,
        // set emailProvider to "smtp"
        if (formSettings.smtp.enabled && !formSettings.emailProvider) {
            formSettings.emailProvider = "smtp";
        }

        // Similarly for resend
        if (formSettings.resend?.enabled && !formSettings.emailProvider) {
            formSettings.emailProvider = "resend";
        }

        if (!formSettings.smtp.authMethod) {
            formSettings.smtp.authMethod = authMethods[0].value;
        }

        originalFormSettings = JSON.parse(JSON.stringify(formSettings));

        maskSmtpPassword = !!formSettings.smtp.username;
        // Use enabled/emailProvider as indicator since apiKey is masked (returned empty) from API
        maskResendApiKey = formSettings.emailProvider === "resend" || !!formSettings.resend?.enabled;
    }

    function reset() {
        formSettings = JSON.parse(JSON.stringify(originalFormSettings || {}));
        // Restore mask state for sensitive fields
        maskSmtpPassword = !!originalFormSettings.smtp?.username;
        maskResendApiKey =
            originalFormSettings.emailProvider === "resend" || !!originalFormSettings.resend?.enabled;
    }
</script>

<SettingsSidebar />

<PageWrapper>
    <header class="page-header">
        <nav class="breadcrumbs">
            <div class="breadcrumb-item">Settings</div>
            <div class="breadcrumb-item">{$pageTitle}</div>
        </nav>
    </header>

    <div class="wrapper">
        <form class="panel" autocomplete="off" on:submit|preventDefault={() => save()}>
            <div class="content txt-xl m-b-base">
                <p>Configure common settings for sending emails.</p>
            </div>

            {#if isLoading}
                <div class="loader" />
            {:else}
                <div class="grid m-b-base">
                    <div class="col-lg-6">
                        <Field class="form-field required" name="meta.senderName" let:uniqueId>
                            <label for={uniqueId}>Sender name</label>
                            <input
                                type="text"
                                id={uniqueId}
                                required
                                bind:value={formSettings.meta.senderName}
                            />
                        </Field>
                    </div>

                    <div class="col-lg-6">
                        <Field class="form-field required" name="meta.senderAddress" let:uniqueId>
                            <label for={uniqueId}>Sender address</label>
                            <input
                                type="email"
                                id={uniqueId}
                                required
                                bind:value={formSettings.meta.senderAddress}
                            />
                        </Field>
                    </div>
                </div>

                <Field class="form-field m-b-base" name="emailProvider" let:uniqueId>
                    <label for={uniqueId}>
                        <span class="txt">Email provider</span>
                        <i
                            class="ri-information-line link-hint"
                            use:tooltip={{
                                text: 'By default PocketBase uses the unix "sendmail" command for sending emails. For better deliverability, use SMTP or Resend.',
                                position: "top",
                            }}
                        />
                    </label>
                    <ObjectSelect
                        id={uniqueId}
                        items={emailProviderOptions}
                        bind:keyOfSelected={formSettings.emailProvider}
                    />
                </Field>

                {#if formSettings.emailProvider === "smtp"}
                    <div transition:slide={{ duration: 150 }}>
                        <div class="grid">
                            <div class="col-lg-4">
                                <Field class="form-field required" name="smtp.host" let:uniqueId>
                                    <label for={uniqueId}>SMTP server host</label>
                                    <input
                                        type="text"
                                        id={uniqueId}
                                        required
                                        bind:value={formSettings.smtp.host}
                                    />
                                </Field>
                            </div>
                            <div class="col-lg-2">
                                <Field class="form-field required" name="smtp.port" let:uniqueId>
                                    <label for={uniqueId}>Port</label>
                                    <input
                                        type="number"
                                        id={uniqueId}
                                        required
                                        bind:value={formSettings.smtp.port}
                                    />
                                </Field>
                            </div>
                            <div class="col-lg-3">
                                <Field class="form-field" name="smtp.username" let:uniqueId>
                                    <label for={uniqueId}>Username</label>
                                    <input
                                        type="text"
                                        id={uniqueId}
                                        bind:value={formSettings.smtp.username}
                                    />
                                </Field>
                            </div>
                            <div class="col-lg-3">
                                <Field class="form-field" name="smtp.password" let:uniqueId>
                                    <label for={uniqueId}>Password</label>
                                    <RedactedPasswordInput
                                        id={uniqueId}
                                        bind:mask={maskSmtpPassword}
                                        bind:value={formSettings.smtp.password}
                                    />
                                </Field>
                            </div>
                        </div>

                        <button
                            type="button"
                            class="btn btn-sm btn-secondary m-t-sm m-b-sm"
                            on:click|preventDefault={() => {
                                showMoreOptions = !showMoreOptions;
                            }}
                        >
                            {#if showMoreOptions}
                                <span class="txt">Hide more options</span>
                                <i class="ri-arrow-up-s-line" />
                            {:else}
                                <span class="txt">Show more options</span>
                                <i class="ri-arrow-down-s-line" />
                            {/if}
                        </button>

                        {#if showMoreOptions}
                            <div class="grid" transition:slide={{ duration: 150 }}>
                                <div class="col-lg-3">
                                    <Field class="form-field" name="smtp.tls" let:uniqueId>
                                        <label for={uniqueId}>TLS encryption</label>
                                        <ObjectSelect
                                            id={uniqueId}
                                            items={tlsOptions}
                                            bind:keyOfSelected={formSettings.smtp.tls}
                                        />
                                    </Field>
                                </div>
                                <div class="col-lg-3">
                                    <Field class="form-field" name="smtp.authMethod" let:uniqueId>
                                        <label for={uniqueId}>AUTH method</label>
                                        <ObjectSelect
                                            id={uniqueId}
                                            items={authMethods}
                                            bind:keyOfSelected={formSettings.smtp.authMethod}
                                        />
                                    </Field>
                                </div>
                                <div class="col-lg-6">
                                    <Field class="form-field" name="smtp.localName" let:uniqueId>
                                        <label for={uniqueId}>
                                            <span class="txt">EHLO/HELO domain</span>
                                            <i
                                                class="ri-information-line link-hint"
                                                use:tooltip={{
                                                    text: "Some SMTP servers, such as the Gmail SMTP-relay, requires a proper domain name in the inital EHLO/HELO exchange and will reject attempts to use localhost.",
                                                    position: "top",
                                                }}
                                            />
                                        </label>
                                        <input
                                            type="text"
                                            id={uniqueId}
                                            placeholder="Default to localhost"
                                            bind:value={formSettings.smtp.localName}
                                        />
                                    </Field>
                                </div>
                                <div class="col-lg-12" />
                            </div>
                        {/if}
                    </div>
                {/if}

                {#if formSettings.emailProvider === "resend"}
                    <div transition:slide={{ duration: 150 }}>
                        <div class="grid">
                            <div class="col-lg-6">
                                <Field class="form-field required" name="resend.apiKey" let:uniqueId>
                                    <label for={uniqueId}>
                                        <span class="txt">Resend API Key</span>
                                        <i
                                            class="ri-information-line link-hint"
                                            use:tooltip={{
                                                text: "You can get your API key from resend.com/api-keys",
                                                position: "top",
                                            }}
                                        />
                                    </label>
                                    <RedactedPasswordInput
                                        id={uniqueId}
                                        bind:mask={maskResendApiKey}
                                        bind:value={formSettings.resend.apiKey}
                                    />
                                </Field>
                            </div>
                        </div>
                    </div>
                {/if}

                <div class="flex m-t-base">
                    <div class="flex-fill" />

                    {#if hasChanges}
                        <button
                            type="button"
                            class="btn btn-transparent btn-hint"
                            disabled={isSaving}
                            on:click={() => reset()}
                        >
                            <span class="txt">Cancel</span>
                        </button>
                        <button
                            type="submit"
                            class="btn btn-expanded"
                            class:btn-loading={isSaving}
                            disabled={!hasChanges || isSaving}
                            on:click={() => save()}
                        >
                            <span class="txt">Save changes</span>
                        </button>
                    {:else}
                        <button
                            type="button"
                            class="btn btn-expanded btn-outline"
                            on:click={() => testPopup?.show()}
                        >
                            <i class="ri-mail-check-line" />
                            <span class="txt">Send test email</span>
                        </button>
                    {/if}
                </div>
            {/if}
        </form>
    </div>
</PageWrapper>

<EmailTestPopup bind:this={testPopup} />
