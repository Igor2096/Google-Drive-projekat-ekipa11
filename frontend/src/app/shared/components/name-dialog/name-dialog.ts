import { Component, ElementRef, afterNextRender, input, output, signal, viewChild } from '@angular/core';
import { validateName } from '../../validators/name.validator';

/** Mali modal sa jednim poljem - koristi se za "Novi folder" i "Preimenuj". */
@Component({
  selector: 'app-name-dialog',
  template: `
    <div class="backdrop" (click)="cancelled.emit()">
      <div class="dialog" role="dialog" aria-modal="true" (click)="$event.stopPropagation()">
        <h2>{{ title() }}</h2>
        <input
          #nameInput
          class="text-input"
          [class.invalid]="touched() && error()"
          [value]="value()"
          (input)="onInput($any($event.target).value)"
          (keydown.enter)="submit()"
          (keydown.escape)="cancelled.emit()"
          maxlength="255"
        />
        @if (touched() && error()) {
          <p class="field-error">{{ error() }}</p>
        }
        @if (serverError()) {
          <p class="field-error">{{ serverError() }}</p>
        }
        <div class="dialog-actions">
          <button class="btn-text" type="button" (click)="cancelled.emit()">Otkaži</button>
          <button class="btn-text primary" type="button" [disabled]="busy()" (click)="submit()">
            {{ confirmLabel() }}
          </button>
        </div>
      </div>
    </div>
  `,
})
export class NameDialog {
  readonly title = input.required<string>();
  readonly confirmLabel = input('Napravi');
  readonly initialValue = input('');
  readonly busy = input(false);
  readonly serverError = input<string | null>(null);

  readonly confirmed = output<string>();
  readonly cancelled = output<void>();

  protected readonly value = signal('');
  protected readonly touched = signal(false);
  protected readonly error = signal<string | null>(null);

  private readonly nameInput = viewChild.required<ElementRef<HTMLInputElement>>('nameInput');

  constructor() {
    afterNextRender(() => {
      this.value.set(this.initialValue());
      this.error.set(validateName(this.initialValue()));
      const el = this.nameInput().nativeElement;
      el.value = this.initialValue();
      el.focus();
      el.select();
    });
  }

  protected onInput(v: string) {
    this.value.set(v);
    this.touched.set(true);
    this.error.set(validateName(v));
  }

  protected submit() {
    this.touched.set(true);
    const err = validateName(this.value());
    this.error.set(err);
    if (!err && !this.busy()) {
      this.confirmed.emit(this.value().trim());
    }
  }
}
