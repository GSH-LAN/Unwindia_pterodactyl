import { Injectable, OnModuleInit } from '@nestjs/common';
import axios, { AxiosRequestConfig, AxiosPromise } from 'axios';


@Injectable()
export class AppService implements OnModuleInit {
  getHello(): string {
    return 'Hello World!';
  }
  onModuleInit() {
    axios.get('http://localhost:1337/configuration/pterodactyl').then(resp => {
      if (resp.data.value === false) {
        console.log(`yeah` + resp.data.value);
      } else {
        process.exit();
      }
    });
    console.log(`The module has been initialized.`);
  }
}