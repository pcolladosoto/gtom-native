import React, { ComponentType } from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { Field, InlineField, InlineSwitch, Input, SecretInput } from '@grafana/ui';
import { GenericOptions } from '../types';
import { QueryEditorModeToggle } from './QueryEditorModeToggle';

type Props = DataSourcePluginOptionsEditorProps<GenericOptions, MySecureJsonData>;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onPasswordReset = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        basicAuthPassword: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        basicAuthPassword: '',
      },
    });
  };

  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        basicAuthPassword: event.target.value,
      },
    });
  };

  const isValidUri = /^(mongodb|mongodb+srv):\/\/(\w+:{0,1}\w*@)?(\S+)(:[0-9]+)?(\/|\/([\w#!:.?+=&%@!\-\/]))?$/.test(
    options.url
  );

  return (
    <>
      <h3 className="page-heading">Database</h3>
      <div className="gf-form-group">
        <div className="gf-form-inline">
          <div className="gf-form">
            <InlineField label="Instance URI">
              <Input
                id="config-editor-uri"
                onChange={(event: ChangeEvent<HTMLInputElement>) => {
                  onOptionsChange({
                    ...options,
                    url: event.target.value,
                  });
                }}
                value={options.url}
                placeholder="mongodb://localhost:27017"
                width={40}
                invalid={!isValidUri}
              />
            </InlineField>
            <InlineField label="Database">
              <Input
                id="config-editor-db"
                onChange={(event: ChangeEvent<HTMLInputElement>) => {
                  onOptionsChange({
                    ...options,
                    jsonData: {
                      database: event.target.value
                    }
                  });
                }}
                value={jsonData.database}
                placeholder="telegrafData"
                width={40}
              />
            </InlineField>
          </div>
        </div>
      </div>

      <h3 className="page-heading">Auth</h3>
      <div className="gf-form-group">
        <div className="gf-form-inline">
          <div className="gf-form">
            <InlineField label="Basic Auth">
              <InlineSwitch
                id="http-settings-basic-auth"
                value={options.basicAuth}
                onChange={(event) => {
                  onOptionsChange({
                    ...options,
                    basicAuth: event!.currentTarget.checked
                  });
                }}
              />
            </InlineField>
          </div>
        </div>
      </div>

      {options.basicAuth && (
        <>
          <h6>Basic Auth Details</h6>
          <div className="gf-form-group">
            <>
              <InlineField label="User">
                <Input
                  id="config-editor-user"
                  onChange={(event: ChangeEvent<HTMLInputElement>) => {
                    onOptionsChange({
                      ...options,
                      basicAuthUser: event.target.value,
                    });
                  }}
                  value={options.basicAuthUser}
                  placeholder="Basic Auth User"
                  width={40}
                />
              </InlineField>
              <InlineField label="Password">
                <SecretInput
                  // isConfigured={secureJsonFields.basicAuthPassword}
                  isConfigured={false}
                  value={secureJsonData?.basicAuthPassword}
                  inputWidth={18}
                  labelWidth={10}
                  placeholder="Basic Auth Password"
                  onReset={onPasswordReset}
                  onChange={onPasswordChange}
                />
              </InlineField>
            </>
          </div>
        </>
      )}

      <h3 className="page-heading">Other</h3>
      <div className="gf-form-group">
        <div className="gf-form-inline">
          <div className="gf-form">
            <InlineField label="Default edit mode">
              <QueryEditorModeToggle
                size="md"
                mode={options.jsonData.defaultEditorMode ?? 'code'}
                onChange={(v) => {
                  onOptionsChange({
                    ...options,
                    jsonData: {
                      ...options.jsonData,
                      defaultEditorMode: v,
                    },
                  });
                }}
              />
            </InlineField>
          </div>
        </div>
      </div>
    </>
  );
}
